package storecenter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	storeServiceConstraintLockKey             = "task-processor:workbench-stores:store-service-phase-e:v1"
	maxStoreServiceConstraintLockTimeout      = 30 * time.Second
	maxStoreServiceConstraintStatementTimeout = 30 * time.Minute
)

var (
	ErrStoreServiceConstraintsUnsupported = errors.New("store service constraints require PostgreSQL")
	ErrStoreServiceConstraintOwnership    = errors.New("store service constraint ownership mismatch")
)

type StoreServiceConstraintOptions struct {
	LockTimeout      time.Duration
	StatementTimeout time.Duration
}

func (options StoreServiceConstraintOptions) Validate() error {
	if options.LockTimeout <= 0 || options.LockTimeout > maxStoreServiceConstraintLockTimeout {
		return fmt.Errorf("constraint lock timeout must be between 1ns and %s", maxStoreServiceConstraintLockTimeout)
	}
	if options.StatementTimeout <= 0 || options.StatementTimeout > maxStoreServiceConstraintStatementTimeout {
		return fmt.Errorf("constraint statement timeout must be between 1ns and %s", maxStoreServiceConstraintStatementTimeout)
	}
	return nil
}

type StoreServiceConstraintReport struct {
	PhaseD               StoreHistoryMigrationReport `json:"phase_d"`
	ConstraintsAdded     int                         `json:"constraints_added"`
	ConstraintsValidated int                         `json:"constraints_validated"`
	RecordStatusNotNull  bool                        `json:"record_status_not_null"`
	ConstraintsApplied   bool                        `json:"constraints_applied"`
}

type storeServiceConstraintDefinition struct {
	Name       string
	Expression string
	Marker     string
}

func storeServiceConstraintDefinitions() []storeServiceConstraintDefinition {
	const markerPrefix = "task-processor/store-service-phase-e/v1/"
	return []storeServiceConstraintDefinition{
		{
			Name:       "workbench_stores_record_status_nn",
			Expression: "record_status IS NOT NULL",
			Marker:     markerPrefix + "record-status-nn",
		},
		{
			Name:       "workbench_stores_record_status_enum",
			Expression: "record_status IN ('provisioning', 'active', 'deleting', 'deleted')",
			Marker:     markerPrefix + "record-status-enum",
		},
		{
			Name:       "workbench_stores_service_status_enum",
			Expression: "service_status IS NULL OR service_status IN ('pending_activation', 'active', 'expired', 'suspended')",
			Marker:     markerPrefix + "service-status-enum",
		},
		{
			Name: "workbench_stores_record_service_shape",
			Expression: `(record_status = 'active' AND service_status IS NOT NULL)
OR (record_status IN ('provisioning', 'deleting', 'deleted')
    AND service_status IS NULL
    AND service_started_at IS NULL
    AND service_expires_at IS NULL)`,
			Marker: markerPrefix + "record-service-shape",
		},
		{
			Name: "workbench_stores_service_timestamp_shape",
			Expression: `(service_status IS NULL AND service_started_at IS NULL AND service_expires_at IS NULL)
OR (service_status = 'pending_activation' AND service_started_at IS NULL AND service_expires_at IS NULL)
OR (service_status IN ('active', 'expired')
    AND service_started_at IS NOT NULL
    AND service_expires_at IS NOT NULL
    AND service_expires_at > service_started_at)
OR (service_status = 'suspended'
    AND ((service_started_at IS NULL AND service_expires_at IS NULL)
         OR (service_started_at IS NOT NULL
             AND service_expires_at IS NOT NULL
             AND service_expires_at > service_started_at)))`,
			Marker: markerPrefix + "service-timestamp-shape",
		},
	}
}

// ApplyConstraints performs the explicit Phase E transition. It is never
// called by AutoMigrate: the operator must select the constraints action and
// provide the same immutable history manifest used by Phase D.
func (migrator *GormStoreHistoryMigrator) ApplyConstraints(ctx context.Context, options StoreServiceConstraintOptions) (StoreServiceConstraintReport, error) {
	var report StoreServiceConstraintReport
	if migrator == nil || migrator.db == nil || migrator.resolver == nil {
		return report, errors.New("store history migrator is required")
	}
	if ctx == nil {
		return report, errors.New("constraint context is required")
	}
	if err := options.Validate(); err != nil {
		return report, err
	}
	if migrator.db.Dialector == nil || migrator.db.Dialector.Name() != "postgres" {
		return report, ErrStoreServiceConstraintsUnsupported
	}

	verifyContext, cancelVerify := context.WithTimeout(ctx, options.StatementTimeout)
	phaseD, err := migrator.Verify(verifyContext)
	cancelVerify()
	report.PhaseD = phaseD
	if err != nil {
		return report, fmt.Errorf("verify Phase D before Store service constraints: %w", err)
	}
	if !phaseD.ReadyForConstraints {
		return report, ErrStoreHistoryRolloutBlocked
	}

	definitions := storeServiceConstraintDefinitions()
	for _, definition := range definitions {
		added, ensureErr := migrator.ensureStoreServiceConstraint(ctx, options, definition)
		if ensureErr != nil {
			return report, ensureErr
		}
		if added {
			report.ConstraintsAdded++
		}
	}
	for _, definition := range definitions {
		validated, validateErr := migrator.validateStoreServiceConstraint(ctx, options, definition)
		if validateErr != nil {
			return report, validateErr
		}
		if validated {
			report.ConstraintsValidated++
		}
	}

	notNull, err := migrator.setStoreRecordStatusNotNull(ctx, options)
	if err != nil {
		return report, err
	}
	report.RecordStatusNotNull = notNull
	report.ConstraintsApplied = notNull
	return report, nil
}

func (migrator *GormStoreHistoryMigrator) ensureStoreServiceConstraint(ctx context.Context, options StoreServiceConstraintOptions, definition storeServiceConstraintDefinition) (bool, error) {
	added := false
	err := migrator.runStoreServiceConstraintStep(ctx, options, func(tx *gorm.DB) error {
		state, found, err := loadStoreServiceConstraintState(tx, definition.Name)
		if err != nil {
			return err
		}
		if found {
			return validateStoreServiceConstraintOwnership(definition, state)
		}
		statement := fmt.Sprintf(`ALTER TABLE "workbench_stores" ADD CONSTRAINT "%s" CHECK (%s) NOT VALID`, definition.Name, definition.Expression)
		if err := tx.Exec(statement).Error; err != nil {
			return fmt.Errorf("add Store service constraint %s: %w", definition.Name, err)
		}
		comment := fmt.Sprintf(`COMMENT ON CONSTRAINT "%s" ON "workbench_stores" IS %s`, definition.Name, postgresStringLiteral(definition.Marker))
		if err := tx.Exec(comment).Error; err != nil {
			return fmt.Errorf("mark Store service constraint %s: %w", definition.Name, err)
		}
		added = true
		return nil
	})
	return added, err
}

func postgresStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (migrator *GormStoreHistoryMigrator) validateStoreServiceConstraint(ctx context.Context, options StoreServiceConstraintOptions, definition storeServiceConstraintDefinition) (bool, error) {
	validated := false
	err := migrator.runStoreServiceConstraintStep(ctx, options, func(tx *gorm.DB) error {
		state, found, err := loadStoreServiceConstraintState(tx, definition.Name)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("Store service constraint %s is missing", definition.Name)
		}
		if err := validateStoreServiceConstraintOwnership(definition, state); err != nil {
			return err
		}
		if state.Validated {
			return nil
		}
		statement := fmt.Sprintf(`ALTER TABLE "workbench_stores" VALIDATE CONSTRAINT "%s"`, definition.Name)
		if err := tx.Exec(statement).Error; err != nil {
			return fmt.Errorf("validate Store service constraint %s: %w", definition.Name, err)
		}
		validated = true
		return nil
	})
	return validated, err
}

func (migrator *GormStoreHistoryMigrator) setStoreRecordStatusNotNull(ctx context.Context, options StoreServiceConstraintOptions) (bool, error) {
	notNull := false
	err := migrator.runStoreServiceConstraintStep(ctx, options, func(tx *gorm.DB) error {
		var state struct {
			NotNull bool `gorm:"column:not_null"`
		}
		result := tx.Raw(`
SELECT attribute.attnotnull AS not_null
FROM pg_attribute AS attribute
JOIN pg_class AS table_row ON table_row.oid = attribute.attrelid
JOIN pg_namespace AS schema_row ON schema_row.oid = table_row.relnamespace
WHERE table_row.relname = 'workbench_stores'
  AND schema_row.nspname = current_schema()
  AND attribute.attname = 'record_status'
  AND attribute.attnum > 0
  AND NOT attribute.attisdropped`).Scan(&state)
		if result.Error != nil {
			return fmt.Errorf("inspect Store record_status nullability: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return errors.New("workbench_stores.record_status is missing")
		}
		if !state.NotNull {
			if err := tx.Exec(`ALTER TABLE "workbench_stores" ALTER COLUMN "record_status" SET NOT NULL`).Error; err != nil {
				return fmt.Errorf("set Store record_status NOT NULL: %w", err)
			}
		}
		notNull = true
		return nil
	})
	return notNull, err
}

func (migrator *GormStoreHistoryMigrator) runStoreServiceConstraintStep(ctx context.Context, options StoreServiceConstraintOptions, operation func(*gorm.DB) error) error {
	return migrator.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lockTimeout := fmt.Sprintf("%dms", max(options.LockTimeout.Milliseconds(), 1))
		statementTimeout := fmt.Sprintf("%dms", max(options.StatementTimeout.Milliseconds(), 1))
		if err := tx.Exec(`SELECT set_config('lock_timeout', ?, true), set_config('statement_timeout', ?, true)`, lockTimeout, statementTimeout).Error; err != nil {
			return fmt.Errorf("set Store service constraint timeouts: %w", err)
		}
		if err := tx.Exec(`SELECT pg_advisory_xact_lock(hashtext(?))`, storeServiceConstraintLockKey).Error; err != nil {
			return fmt.Errorf("acquire Store service constraint lock: %w", err)
		}
		return operation(tx)
	})
}

type storeServiceConstraintState struct {
	ConstraintType string `gorm:"column:constraint_type"`
	Validated      bool   `gorm:"column:validated"`
	NoInherit      bool   `gorm:"column:no_inherit"`
	Marker         string `gorm:"column:marker"`
}

func loadStoreServiceConstraintState(db *gorm.DB, name string) (storeServiceConstraintState, bool, error) {
	var state storeServiceConstraintState
	result := db.Raw(`
SELECT constraint_row.contype::text AS constraint_type,
       constraint_row.convalidated AS validated,
       constraint_row.connoinherit AS no_inherit,
       COALESCE(obj_description(constraint_row.oid, 'pg_constraint'), '') AS marker
FROM pg_constraint AS constraint_row
JOIN pg_class AS table_row ON table_row.oid = constraint_row.conrelid
JOIN pg_namespace AS schema_row ON schema_row.oid = table_row.relnamespace
WHERE constraint_row.conname = ?
  AND table_row.relname = 'workbench_stores'
  AND schema_row.nspname = current_schema()`, name).Scan(&state)
	if result.Error != nil {
		return state, false, fmt.Errorf("inspect Store service constraint %s: %w", name, result.Error)
	}
	return state, result.RowsAffected == 1, nil
}

func validateStoreServiceConstraintOwnership(definition storeServiceConstraintDefinition, state storeServiceConstraintState) error {
	if state.ConstraintType != "c" || state.NoInherit || state.Marker != definition.Marker {
		return fmt.Errorf("%w: %s", ErrStoreServiceConstraintOwnership, definition.Name)
	}
	return nil
}
