package accountprofile

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	store "task-processor/internal/integration/persistence/accountprofile"
	platformmigration "task-processor/internal/platform/database/migration"
)

const versionTableRelation = "public.goose_account_profile_version"

// The profile shape is organization-scoped now. A new baseline intentionally
// invalidates databases initialized with the old user-only shape; greenfield
// installs must create the current schema, while stale state must be recreated.
const baselineVersion = int64(2026092102)
const auditVersion = int64(2026092103)

func Migrations() []*goose.Migration {
	return []*goose.Migration{
		goose.NewGoMigration(baselineVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			return store.InstallSchemaTx(ctx, tx)
		}}, nil),
		goose.NewGoMigration(auditVersion, &goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS public.account_business_profile_audit_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id VARCHAR(128) NOT NULL,
    actor_id VARCHAR(128) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    operation VARCHAR(32) NOT NULL,
    version BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_business_profile_audit_ids_check CHECK (octet_length(organization_id) BETWEEN 1 AND 128 AND organization_id = btrim(organization_id) AND octet_length(actor_id) BETWEEN 1 AND 128 AND actor_id = btrim(actor_id) AND octet_length(user_id) BETWEEN 1 AND 128 AND user_id = btrim(user_id)),
    CONSTRAINT account_business_profile_audit_operation_check CHECK (operation = 'update'),
    CONSTRAINT account_business_profile_audit_version_check CHECK (version > 0)
)`)
			return err
		}}, nil),
	}
}

func Migrate(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("account profile schema requires postgres")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get account profile database: %w", err)
	}
	runner, err := platformmigration.NewWithVersionTable(goose.DialectPostgres, sqlDB, versionTableRelation, Migrations()...)
	if err != nil {
		return fmt.Errorf("create account profile migration runner: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("initialize account profile schema: %w", err)
	}
	return store.VerifySchema(ctx, db)
}
