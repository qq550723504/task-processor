package storecenter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/shared/resilience"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrStoreHistoryRolloutBlocked = errors.New("store service history rollout is blocked")

const maxStoreHistoryBackfillBatchSize = 1000

const (
	historyResolutionNotApplicableNew = "not_applicable_new"
	historySourceStoreCreate          = "store-create"
	storeHistoryRetryAttempts         = 8
	storeHistoryRetryBudget           = 15 * time.Second
)

// StoreHistoryMigrationReport exposes the exact Phase D/F blocker counters.
// No-authoritative-source mode can only produce confirmed-absent evidence;
// found/unavailable are retained so rollout tooling cannot silently omit them.
type StoreHistoryMigrationReport struct {
	ScannedCount                 int64 `json:"scanned_count"`
	UpdatedCount                 int64 `json:"updated_count"`
	HistoryFoundCount            int64 `json:"history_found_count"`
	HistoryConfirmedAbsentCount  int64 `json:"history_confirmed_absent_count"`
	HistoryNotApplicableCount    int64 `json:"history_not_applicable_count"`
	HistoryUnavailableCount      int64 `json:"history_unavailable_count"`
	HistoryErrorCount            int64 `json:"history_error_count"`
	HistorySnapshotConflictCount int64 `json:"history_snapshot_conflict_count"`
	HistoryHandoffBacklogCount   int64 `json:"history_handoff_backlog_count"`
	UnresolvedCount              int64 `json:"unresolved_count"`
	InvalidStateCount            int64 `json:"invalid_state_count"`
	ReadyForConstraints          bool  `json:"ready_for_constraints"`
}

type GormStoreHistoryMigrator struct {
	db       *gorm.DB
	resolver *NoAuthoritativeHistorySourceResolver
	actor    string
	now      func() time.Time
}

func NewGormStoreHistoryMigrator(db *gorm.DB, manifest NoAuthoritativeHistorySourceManifest, actor string, now func() time.Time) (*GormStoreHistoryMigrator, error) {
	if db == nil || now == nil {
		return nil, errors.New("store history migration dependencies are required")
	}
	actor, err := validateOpaqueIdentity("migration actor", actor, MaxSubjectBytes)
	if err != nil {
		return nil, err
	}
	resolver, err := NewNoAuthoritativeHistorySourceResolver(manifest)
	if err != nil {
		return nil, err
	}
	return &GormStoreHistoryMigrator{db: db, resolver: resolver, actor: actor, now: now}, nil
}

func (migrator *GormStoreHistoryMigrator) BackfillBatch(ctx context.Context, batchSize int) (StoreHistoryMigrationReport, error) {
	var report StoreHistoryMigrationReport
	if migrator == nil || migrator.db == nil || migrator.resolver == nil {
		return report, errors.New("store history migrator is required")
	}
	if batchSize <= 0 || batchSize > maxStoreHistoryBackfillBatchSize {
		return report, errors.New("store history backfill batch size is invalid")
	}

	var ids []string
	err := migrator.db.WithContext(ctx).Unscoped().Model(&workbenchStoreRecord{}).
		Where("record_status IS NULL OR (deleted_at IS NULL AND lifecycle_status IN ? AND service_history_resolution_status IS NULL)", []string{string(StoreStatusActive), string(StoreStatusDisabled)}).
		Order("id ASC").Limit(batchSize).Pluck("id", &ids).Error
	if err != nil {
		return report, fmt.Errorf("list Store history backfill candidates: %w", err)
	}

	for _, id := range ids {
		outcome, err := migrator.backfillOne(ctx, id)
		report.ScannedCount++
		if err != nil {
			report.HistoryErrorCount++
			return report, fmt.Errorf("backfill Store history %s: %w", id, err)
		}
		if outcome.updated {
			report.UpdatedCount++
		}
		if outcome.confirmedAbsent {
			report.HistoryConfirmedAbsentCount++
		}
	}
	return report, nil
}

type storeHistoryBackfillOutcome struct {
	updated         bool
	confirmedAbsent bool
}

func (migrator *GormStoreHistoryMigrator) backfillOne(ctx context.Context, storeID string) (storeHistoryBackfillOutcome, error) {
	var committed storeHistoryBackfillOutcome
	retryContext, cancel := context.WithTimeout(ctx, storeHistoryRetryBudget)
	defer cancel()
	err := resilience.Retry(retryContext, resilience.RetryConfig{
		MaxAttempts:         storeHistoryRetryAttempts,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            250 * time.Millisecond,
		Multiplier:          2,
		RandomizationFactor: 0.2,
		IsRetryable:         isStoreHistoryTransientConcurrencyError,
	}, func(attemptContext context.Context) error {
		attemptOutcome, attemptErr := migrator.backfillOneAttempt(attemptContext, storeID)
		if attemptErr == nil {
			committed = attemptOutcome
		}
		return attemptErr
	})
	return committed, err
}

func (migrator *GormStoreHistoryMigrator) backfillOneAttempt(ctx context.Context, storeID string) (storeHistoryBackfillOutcome, error) {
	var outcome storeHistoryBackfillOutcome
	err := migrator.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record workbenchStoreRecord
		err := tx.WithContext(ctx).Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", storeID).Take(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lock Store migration row: %w", err)
		}
		if !storeNeedsHistoryBackfill(record) {
			return nil
		}

		state, historyRequired, err := historyBackfillState(record)
		if err != nil {
			return err
		}
		if hasAnyHistoryEvidence(record) {
			return errors.New("partial or unexpected Store history evidence")
		}

		occurredAt := migrator.now().UTC()
		if occurredAt.IsZero() {
			return errors.New("store history migration time is required")
		}
		if occurredAt.Before(record.UpdatedAt) {
			occurredAt = record.UpdatedAt.UTC()
		}
		updates := state.columns()
		if historyRequired {
			resolution, freeze, resolveErr := migrator.resolver.Resolve(ctx, StoreSnapshot{ID: record.ID, OrganizationID: record.OrganizationID})
			if resolveErr != nil {
				return resolveErr
			}
			if validateErr := ValidateLegacyServiceHistoryFreeze(resolution, freeze); validateErr != nil {
				return validateErr
			}
			if resolution.Status != HistoryConfirmedAbsent {
				return ErrInvalidLegacyHistoryResolution
			}
			updates["service_history_resolution_status"] = string(resolution.Status)
			updates["service_history_source_identity"] = resolution.SourceIdentity
			updates["service_history_snapshot_token"] = resolution.SourceSnapshotToken
			updates["service_history_resolved_at"] = occurredAt
			outcome.confirmedAbsent = true
		}

		updates["version"] = gorm.Expr("version + ?", 1)
		updates["updated_by"] = migrator.actor
		updates["updated_at"] = occurredAt
		result := tx.WithContext(ctx).Unscoped().Model(&workbenchStoreRecord{}).
			Where("id = ? AND version = ?", record.ID, record.Version).
			Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("apply Store history backfill: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return ErrVersionConflict
		}
		outcome.updated = true
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	return outcome, err
}

func isStoreHistoryTransientConcurrencyError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrVersionConflict) {
		return true
	}
	var stateError interface{ SQLState() string }
	if errors.As(err, &stateError) {
		switch stateError.SQLState() {
		case "40001", "40P01", "55P03":
			return true
		default:
			return false
		}
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_busy") || strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked")
}

func (migrator *GormStoreHistoryMigrator) Verify(ctx context.Context) (StoreHistoryMigrationReport, error) {
	var report StoreHistoryMigrationReport
	if migrator == nil || migrator.db == nil || migrator.resolver == nil {
		return report, errors.New("store history migrator is required")
	}
	var records []workbenchStoreRecord
	if err := migrator.db.WithContext(ctx).Unscoped().Order("id ASC").Find(&records).Error; err != nil {
		return report, fmt.Errorf("load Store history verification rows: %w", err)
	}
	report.ScannedCount = int64(len(records))
	for _, record := range records {
		state, mapped := expandedStateFromRecord(record)
		if !mapped {
			report.UnresolvedCount++
			continue
		}
		if err := ValidateStoreServiceState(state); err != nil || !historyStateMatchesLegacyLifecycle(record, state) {
			report.InvalidStateCount++
			continue
		}
		if hasAnyHistoryEvidence(record) {
			if completeConfirmedAbsentEvidence(record, migrator.resolver.sourceIdentity(), migrator.resolver.snapshotToken()) {
				report.HistoryConfirmedAbsentCount++
			} else if completeNewStoreHistoryEvidence(record) {
				report.HistoryNotApplicableCount++
			} else {
				report.HistorySnapshotConflictCount++
			}
		} else if state.RecordStatus == RecordStatusActive {
			report.UnresolvedCount++
		}
	}
	report.ReadyForConstraints = report.UnresolvedCount == 0 &&
		report.InvalidStateCount == 0 &&
		report.HistoryUnavailableCount == 0 &&
		report.HistoryErrorCount == 0 &&
		report.HistorySnapshotConflictCount == 0 &&
		report.HistoryHandoffBacklogCount == 0
	if !report.ReadyForConstraints {
		return report, fmt.Errorf("%w: unresolved=%d invalid_state=%d snapshot_conflict=%d", ErrStoreHistoryRolloutBlocked, report.UnresolvedCount, report.InvalidStateCount, report.HistorySnapshotConflictCount)
	}
	return report, nil
}

func storeNeedsHistoryBackfill(record workbenchStoreRecord) bool {
	if record.RecordStatus == nil {
		return true
	}
	return !record.DeletedAt.Valid &&
		(record.LifecycleStatus == string(StoreStatusActive) || record.LifecycleStatus == string(StoreStatusDisabled)) &&
		record.ServiceHistoryResolution == nil
}

func historyBackfillState(record workbenchStoreRecord) (StoreServiceState, bool, error) {
	if record.DeletedAt.Valid {
		return StoreServiceState{RecordStatus: RecordStatusDeleted}, false, nil
	}
	if state, mapped := expandedStateFromRecord(record); mapped {
		if err := ValidateStoreServiceState(state); err != nil || !historyStateMatchesLegacyLifecycle(record, state) {
			return StoreServiceState{}, false, ErrInvalidServiceState
		}
		return state, state.RecordStatus == RecordStatusActive, nil
	}
	switch LifecycleStatus(record.LifecycleStatus) {
	case StoreStatusProvisioning:
		return StoreServiceState{RecordStatus: RecordStatusProvisioning}, false, nil
	case StoreStatusActive:
		return StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusPendingActivation}, true, nil
	case StoreStatusDisabled:
		return StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusSuspended}, true, nil
	case StoreStatusDeleting:
		return StoreServiceState{RecordStatus: RecordStatusDeleting}, false, nil
	default:
		return StoreServiceState{}, false, ErrInvalidServiceState
	}
}

func historyStateMatchesLegacyLifecycle(record workbenchStoreRecord, state StoreServiceState) bool {
	if record.DeletedAt.Valid {
		return state.RecordStatus == RecordStatusDeleted && state.ServiceStatus == "" && state.StartedAt == nil && state.ExpiresAt == nil
	}
	switch LifecycleStatus(record.LifecycleStatus) {
	case StoreStatusProvisioning:
		return state.RecordStatus == RecordStatusProvisioning
	case StoreStatusDeleting:
		return state.RecordStatus == RecordStatusDeleting
	case StoreStatusDisabled:
		return state.RecordStatus == RecordStatusActive && state.ServiceStatus == ServiceStatusSuspended
	case StoreStatusActive:
		return state.RecordStatus == RecordStatusActive && state.ServiceStatus != ServiceStatusSuspended
	default:
		return false
	}
}

func hasAnyHistoryEvidence(record workbenchStoreRecord) bool {
	return record.ServiceHistoryResolution != nil || record.ServiceHistorySource != nil || record.ServiceHistoryToken != nil || record.ServiceHistoryResolvedAt != nil
}

func completeConfirmedAbsentEvidence(record workbenchStoreRecord, sourceIdentity, token string) bool {
	return record.ServiceHistoryResolution != nil && *record.ServiceHistoryResolution == string(HistoryConfirmedAbsent) &&
		record.ServiceHistorySource != nil && *record.ServiceHistorySource == sourceIdentity &&
		record.ServiceHistoryToken != nil && *record.ServiceHistoryToken == token &&
		record.ServiceHistoryResolvedAt != nil && !record.ServiceHistoryResolvedAt.IsZero()
}

func completeNewStoreHistoryEvidence(record workbenchStoreRecord) bool {
	return record.ServiceHistoryResolution != nil && *record.ServiceHistoryResolution == historyResolutionNotApplicableNew &&
		record.ServiceHistorySource != nil && *record.ServiceHistorySource == historySourceStoreCreate &&
		record.ServiceHistoryToken != nil && *record.ServiceHistoryToken == record.CreateRequestFingerprint &&
		record.ServiceHistoryResolvedAt != nil && record.ServiceHistoryResolvedAt.Equal(record.CreatedAt)
}
