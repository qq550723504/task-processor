package listingsubscription

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	moderncsqlite "modernc.org/sqlite"
)

// NewGormUsageLedger creates the durable, transaction-backed usage ledger.
func NewGormUsageLedger(repo *GormRepository) UsageLedger {
	return &gormUsageLedger{repo: repo}
}

type gormUsageLedger struct {
	repo *GormRepository
}

func (l *gormUsageLedger) Reserve(ctx context.Context, input ReserveUsageInput) (ReserveUsageResult, error) {
	input, err := NormalizeAndValidateReserveUsageInput(input)
	if err != nil {
		return ReserveUsageResult{}, err
	}
	if input.OccurredAt.IsZero() {
		// Resolve an idempotent replay before deriving a time-sensitive billing
		// period. A retry can cross a month boundary; the persisted event owns
		// the canonical period and occurrence timestamp.
		var existing usageEventRow
		lookupErr := l.repo.db.WithContext(ctx).Where("tenant_id = ? AND idempotency_key = ?", input.TenantID, input.IdempotencyKey).Take(&existing).Error
		if lookupErr == nil {
			comparison := input
			comparison.PeriodKey = existing.PeriodKey
			if !usageEventMatchesReserveInput(usageEventFromRow(existing), comparison) {
				return ReserveUsageResult{}, &UsageDuplicateIdentityError{TenantID: input.TenantID, IdempotencyKey: input.IdempotencyKey}
			}
			var replay ReserveUsageResult
			err := l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				var current usageEventRow
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND idempotency_key = ?", input.TenantID, input.IdempotencyKey).Take(&current).Error; err != nil {
					return err
				}
				comparison.PeriodKey = current.PeriodKey
				if !usageEventMatchesReserveInput(usageEventFromRow(current), comparison) {
					return &UsageDuplicateIdentityError{TenantID: input.TenantID, IdempotencyKey: input.IdempotencyKey}
				}
				return l.reserveResultForExisting(tx, current, &replay)
			})
			return replay, err
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return ReserveUsageResult{}, lookupErr
		}
		input.OccurredAt = time.Now().UTC()
	}
	if input.PeriodKey, err = canonicalUsagePeriodKey(input.Metric, input.PeriodKey, input.OccurredAt); err != nil {
		return ReserveUsageResult{}, err
	}

	var result ReserveUsageResult
	for attempt := 0; attempt < 20; attempt++ {
		result = ReserveUsageResult{}
		err = l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing usageEventRow
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND idempotency_key = ?", input.TenantID, input.IdempotencyKey).Take(&existing).Error
			if err == nil {
				if !usageEventMatchesReserveInput(usageEventFromRow(existing), input) {
					return &UsageDuplicateIdentityError{TenantID: input.TenantID, IdempotencyKey: input.IdempotencyKey}
				}
				return l.reserveResultForExisting(tx, existing, &result)
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			entitlement, err := loadEffectiveUsageEntitlement(tx, input.TenantID, input.ModuleCode)
			if err != nil {
				return err
			}
			effectiveEntitlement, err := entitlement.toEntitlement()
			if err != nil {
				return err
			}
			allowed, _ := evaluateEntitlement(effectiveEntitlement, time.Now().UTC())
			if !allowed {
				return ErrSubscriptionRequired
			}
			limit, err := usageLimit(entitlement, input.Metric)
			if err != nil {
				return err
			}
			bucket, err := loadOrCreateUsageBucket(tx, input)
			if err != nil {
				return err
			}
			reservedForQuota := bucket.Reserved
			if input.Metric == usageMetricStorageBytesCurrent && input.Quantity > 0 {
				reservedForQuota, err = sumPositiveStorageReservations(tx, input)
				if err != nil {
					return err
				}
			}
			if err := validateUsageReservation(input, bucket, limit, reservedForQuota); err != nil {
				return err
			}
			updatedReserved, ok := addUsage(bucket.Reserved, input.Quantity)
			if !ok {
				return &UsageValidationError{Field: "quantity"}
			}

			metadata, err := json.Marshal(input.Metadata)
			if err != nil {
				return err
			}
			event := usageEventRow{
				EventID: uuid.NewString(), TenantID: input.TenantID, ModuleCode: input.ModuleCode,
				Metric: input.Metric, Quantity: input.Quantity, PeriodKey: input.PeriodKey,
				SourceType: input.SourceType, SourceID: input.SourceID, IdempotencyKey: input.IdempotencyKey,
				Status: string(UsageEventReserved), OccurredAt: input.OccurredAt, Metadata: string(metadata),
			}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
			if err := tx.Model(&usageBucketRow{}).
				Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", input.TenantID, input.ModuleCode, usageBucketPeriodKey(input.Metric, input.PeriodKey), input.Metric).
				Updates(map[string]any{"reserved": updatedReserved, "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
			if err := tx.Create(&usageEventOutboxRow{EventID: event.EventID, Status: "reserved"}).Error; err != nil {
				return err
			}
			e := usageEventFromRow(event)
			if input.Metric == usageMetricStorageBytesCurrent {
				snapshot := bucket.Committed
				e.StorageSnapshot = &snapshot
			}
			result = ReserveUsageResult{Event: e, Limit: limit, CommittedUsage: bucket.Committed, ReservedUsage: updatedReserved}
			return nil
		})
		if err == nil || !isRetryableUsageLedgerError(err) || attempt == 19 {
			break
		}
		select {
		case <-ctx.Done():
			return ReserveUsageResult{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 5 * time.Millisecond):
		}
	}
	if err == nil {
		return result, nil
	}

	// A concurrent insert can win after the initial lookup. The unique identity
	// is authoritative, so load it after the failed transaction without changing
	// bucket state a second time.
	var existing usageEventRow
	if lookupErr := l.repo.db.WithContext(ctx).Where("tenant_id = ? AND idempotency_key = ?", input.TenantID, input.IdempotencyKey).Take(&existing).Error; lookupErr == nil {
		if resultErr := l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if !usageEventMatchesReserveInput(usageEventFromRow(existing), input) {
				return &UsageDuplicateIdentityError{TenantID: input.TenantID, IdempotencyKey: input.IdempotencyKey}
			}
			return l.reserveResultForExisting(tx, existing, &result)
		}); resultErr == nil {
			return result, nil
		} else {
			return ReserveUsageResult{}, resultErr
		}
	}
	return ReserveUsageResult{}, err
}

func isRetryableUsageLedgerError(err error) bool {
	var sqliteErr *moderncsqlite.Error
	return errors.As(err, &sqliteErr) && isRetryableSQLiteCode(sqliteErr.Code())
}

func isRetryableSQLiteCode(code int) bool {
	baseCode := code & 0xff
	return baseCode == 5 || baseCode == 6 // SQLITE_BUSY or SQLITE_LOCKED, including extended codes.
}

func (l *gormUsageLedger) Commit(ctx context.Context, eventID string) (UsageEvent, error) {
	return l.transitionReservedEvent(ctx, eventID, UsageEventCommitted, "")
}

func (l *gormUsageLedger) Release(ctx context.Context, eventID, reason string) (UsageEvent, error) {
	return l.transitionReservedEvent(ctx, eventID, UsageEventReleased, reason)
}

func (l *gormUsageLedger) Reverse(ctx context.Context, eventID, idempotencyKey, reason string) (UsageEvent, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return UsageEvent{}, &UsageValidationError{Field: "idempotency_key"}
	}
	var event UsageEvent
	err := l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var source usageEventRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", eventID).Take(&source).Error; err != nil {
			return err
		}
		if source.Status != string(UsageEventCommitted) {
			return ValidateUsageEventTransition(UsageEventStatus(source.Status), UsageEventReversed)
		}
		var sourceOutbox usageEventOutboxRow
		sourceOutboxErr := tx.Where("event_id = ?", source.EventID).Take(&sourceOutbox).Error
		if sourceOutboxErr != nil && !errors.Is(sourceOutboxErr, gorm.ErrRecordNotFound) {
			return sourceOutboxErr
		}
		sourceDelivered := sourceOutboxErr != nil || !usageOutboxUndelivered(sourceOutbox.Status)
		if sourceOutboxErr == nil && sourceOutbox.Status == "failed" {
			return ErrUsageReversalDeliveryUnresolved
		}
		if sourceOutboxErr == nil && sourceOutbox.Status == "in_flight" {
			return ErrUsageReversalDeliveryUnresolved
		}
		laterDelivered, err := hasLaterDeliveredStorageSnapshot(tx, source)
		if err != nil {
			return err
		}
		if source.Metric != usageMetricStorageBytesCurrent && sourceDelivered {
			return ErrUsageReversalProjectionUnsupported
		}
		var priorReversal usageEventRow
		if err := tx.Where("reversal_of = ?", source.EventID).Take(&priorReversal).Error; err == nil {
			event = usageEventFromRow(priorReversal)
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing usageEventRow
		if err := tx.Where("tenant_id = ? AND idempotency_key = ?", source.TenantID, idempotencyKey).Take(&existing).Error; err == nil {
			if existing.ReversalOf == source.EventID {
				event = usageEventFromRow(existing)
				return nil
			}
			return &UsageDuplicateIdentityError{TenantID: source.TenantID, IdempotencyKey: idempotencyKey}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		quantity, ok := negateUsage(source.Quantity)
		if !ok {
			return &UsageValidationError{Field: "quantity"}
		}
		bucket, err := loadUsageBucket(tx, source.TenantID, source.ModuleCode, source.PeriodKey, source.Metric)
		if err != nil {
			return err
		}
		committed, ok := addUsage(bucket.Committed, quantity)
		if !ok || validateUsageBucketTotals(source.Metric, committed, bucket.Reserved) != nil {
			return &UsageValidationError{Field: "usage"}
		}
		metadata, err := redactedUsageMetadata(reason)
		if err != nil {
			return err
		}
		var storageSnapshot *int64
		var storageSnapshotAt *time.Time
		if source.Metric == usageMetricStorageBytesCurrent {
			storageSnapshot = &committed
			now := time.Now().UTC()
			storageSnapshotAt = &now
		}
		reversal := usageEventRow{
			EventID: uuid.NewString(), TenantID: source.TenantID, ModuleCode: source.ModuleCode,
			Metric: source.Metric, Quantity: quantity, PeriodKey: source.PeriodKey,
			SourceType: source.SourceType, SourceID: source.SourceID, IdempotencyKey: idempotencyKey,
			Status: string(UsageEventReversed), OccurredAt: time.Now().UTC(), StorageSnapshot: storageSnapshot, StorageSnapshotAt: storageSnapshotAt, ReversalOf: source.EventID, Metadata: metadata,
		}
		if err := tx.Create(&reversal).Error; err != nil {
			return err
		}
		if err := tx.Model(&usageBucketRow{}).Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", source.TenantID, source.ModuleCode, usageBucketPeriodKey(source.Metric, source.PeriodKey), source.Metric).
			Updates(map[string]any{"committed": committed, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		reversalOutboxStatus := "cancelled"
		if (sourceDelivered || laterDelivered) && source.Metric == usageMetricStorageBytesCurrent {
			reversalOutboxStatus = "pending"
		} else if sourceOutboxErr == nil {
			if err := tx.Model(&usageEventOutboxRow{}).Where("event_id = ? AND status IN ?", source.EventID, []string{"reserved", "pending", "failed"}).Update("status", "cancelled").Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&usageEventOutboxRow{EventID: reversal.EventID, Status: reversalOutboxStatus}).Error; err != nil {
			return err
		}
		event = usageEventFromRow(reversal)
		if source.Metric == usageMetricStorageBytesCurrent {
			event.StorageSnapshot = cloneUsageInt64Pointer(storageSnapshot)
		}
		return nil
	})
	return event, err
}

func hasLaterDeliveredStorageSnapshot(tx *gorm.DB, source usageEventRow) (bool, error) {
	var count int64
	query := tx.Table("saas_usage_events AS e").
		Joins("JOIN saas_usage_event_outbox AS o ON o.event_id = e.event_id").
		Where("e.event_id <> ? AND e.tenant_id = ? AND e.module_code = ? AND e.metric = ? AND e.status IN ? AND o.status NOT IN ?", source.EventID, source.TenantID, source.ModuleCode, usageMetricStorageBytesCurrent, []string{"committed", "reversed"}, []string{"reserved", "pending", "cancelled", "failed"})
	if source.StorageSnapshotAt != nil {
		query = query.Where("e.storage_snapshot_at > ? OR (e.storage_snapshot_at IS NULL AND e.created_at > ?)", *source.StorageSnapshotAt, source.CreatedAt)
	} else {
		query = query.Where("e.created_at > ?", source.CreatedAt)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

func (l *gormUsageLedger) Get(ctx context.Context, tenantID, idempotencyKey string) (*UsageEvent, error) {
	var row usageEventRow
	err := l.repo.db.WithContext(ctx).Where("tenant_id = ? AND idempotency_key = ?", strings.TrimSpace(tenantID), strings.TrimSpace(idempotencyKey)).Take(&row).Error
	if err != nil {
		return nil, err
	}
	event := usageEventFromRow(row)
	return &event, nil
}

func (l *gormUsageLedger) ListPendingOutbox(ctx context.Context, limit int) ([]UsageOutboxItem, error) {
	if limit <= 0 {
		return []UsageOutboxItem{}, nil
	}
	now := time.Now().UTC()
	var rows []usageEventOutboxRow
	err := l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", "pending", now).Order("COALESCE(next_attempt_at, created_at) ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		if err := tx.Model(&usageEventOutboxRow{}).Where("id IN ? AND status = ?", ids, "pending").Updates(map[string]any{"status": "in_flight", "attempts": gorm.Expr("attempts + ?", 1), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		for i := range rows {
			rows[i].Status = "in_flight"
			rows[i].Attempts++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	items := make([]UsageOutboxItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, UsageOutboxItem{ID: row.ID, EventID: row.EventID, Destination: row.Destination, Status: row.Status, Attempts: row.Attempts, NextAttemptAt: row.NextAttemptAt, LastError: row.LastError, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	return items, nil
}

func (l *gormUsageLedger) transitionReservedEvent(ctx context.Context, eventID string, target UsageEventStatus, reason string) (UsageEvent, error) {
	var event UsageEvent
	err := l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row usageEventRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", eventID).Take(&row).Error; err != nil {
			return err
		}
		if row.Status == string(target) {
			event = usageEventFromRow(row)
			return nil
		}
		if row.Status != string(UsageEventReserved) {
			return ValidateUsageEventTransition(UsageEventStatus(row.Status), target)
		}
		bucket, err := loadUsageBucket(tx, row.TenantID, row.ModuleCode, row.PeriodKey, row.Metric)
		if err != nil {
			return err
		}
		reserved, ok := addUsage(bucket.Reserved, -row.Quantity)
		if !ok {
			return &UsageValidationError{Field: "usage"}
		}
		updates := map[string]any{"reserved": reserved, "updated_at": time.Now().UTC()}
		var storageSnapshotValue *int64
		if target == UsageEventCommitted {
			committed, ok := addUsage(bucket.Committed, row.Quantity)
			if !ok {
				return &UsageValidationError{Field: "quantity"}
			}
			updates["committed"] = committed
			if row.Metric == usageMetricStorageBytesCurrent {
				storageSnapshotValue = &committed
			}
		}
		committed := bucket.Committed
		if value, ok := updates["committed"].(int64); ok {
			committed = value
		}
		if err := validateUsageBucketTotals(row.Metric, committed, reserved); err != nil {
			return err
		}
		if err := tx.Model(&usageBucketRow{}).Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", row.TenantID, row.ModuleCode, usageBucketPeriodKey(row.Metric, row.PeriodKey), row.Metric).Updates(updates).Error; err != nil {
			return err
		}
		row.Status = string(target)
		if storageSnapshotValue != nil {
			row.StorageSnapshot = cloneUsageInt64Pointer(storageSnapshotValue)
			snapshotAt := time.Now().UTC()
			row.StorageSnapshotAt = &snapshotAt
		}
		row.UpdatedAt = time.Now().UTC()
		eventUpdates := map[string]any{"status": row.Status, "updated_at": row.UpdatedAt}
		if storageSnapshotValue != nil {
			eventUpdates["storage_snapshot"] = *row.StorageSnapshot
			eventUpdates["storage_snapshot_at"] = *row.StorageSnapshotAt
		}
		if err := tx.Model(&usageEventRow{}).Where("event_id = ?", row.EventID).Updates(eventUpdates).Error; err != nil {
			return err
		}
		if target == UsageEventReleased {
			payload, err := redactedUsageMetadata(reason)
			if err != nil {
				return err
			}
			if err := tx.Create(&auditLogRow{TenantID: row.TenantID, ModuleCode: row.ModuleCode, Action: "usage_released", Payload: payload}).Error; err != nil {
				return err
			}
			if err := tx.Model(&usageEventOutboxRow{}).Where("event_id = ? AND status IN ?", row.EventID, []string{"reserved", "pending"}).Update("status", "cancelled").Error; err != nil {
				return err
			}
		} else if target == UsageEventCommitted {
			if err := tx.Model(&usageEventOutboxRow{}).Where("event_id = ? AND status = ?", row.EventID, "reserved").Update("status", "pending").Error; err != nil {
				return err
			}
		}
		event = usageEventFromRow(row)
		if row.Metric == usageMetricStorageBytesCurrent {
			event.StorageSnapshot = cloneUsageInt64Pointer(row.StorageSnapshot)
			event.StorageSnapshotAt = cloneUsageTimePointer(row.StorageSnapshotAt)
		}
		return nil
	})
	return event, err
}

func (l *gormUsageLedger) reserveResultForExisting(tx *gorm.DB, event usageEventRow, result *ReserveUsageResult) error {
	var current usageEventRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", event.EventID).Take(&current).Error; err != nil {
		return err
	}
	event = current
	entitlement, err := loadEffectiveUsageEntitlement(tx, event.TenantID, event.ModuleCode)
	if err != nil {
		return err
	}
	bucket, err := loadUsageBucket(tx, event.TenantID, event.ModuleCode, event.PeriodKey, event.Metric)
	if err != nil {
		return err
	}
	limit, err := usageLimit(entitlement, event.Metric)
	if err != nil {
		return err
	}
	e := usageEventFromRow(event)
	if event.Metric == usageMetricStorageBytesCurrent {
		e.StorageSnapshot = cloneUsageInt64Pointer(event.StorageSnapshot)
	}
	*result = ReserveUsageResult{Event: e, Existing: true, CommittedUsage: bucket.Committed, ReservedUsage: bucket.Reserved, Limit: limit}
	return nil
}

func loadUsageEntitlement(tx *gorm.DB, tenantID, moduleCode string) (tenantEntitlementRow, error) {
	var entitlement tenantEntitlementRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND module_code = ?", tenantID, moduleCode).Take(&entitlement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tenantEntitlementRow{}, ErrEntitlementNotFound
	}
	return entitlement, err
}

func loadEffectiveUsageEntitlement(tx *gorm.DB, tenantID, moduleCode string) (tenantEntitlementRow, error) {
	entitlement, err := loadUsageEntitlement(tx, tenantID, moduleCode)
	if err == nil || moduleCode != ModuleOSSStorage || !errors.Is(err, ErrEntitlementNotFound) {
		return entitlement, err
	}
	studio, err := loadUsageEntitlement(tx, tenantID, ModuleStudio)
	if err != nil {
		return tenantEntitlementRow{}, err
	}
	studio.ModuleCode = ModuleOSSStorage
	studio.LimitsJSON = "{}"
	return studio, nil
}

func loadOrCreateUsageBucket(tx *gorm.DB, input ReserveUsageInput) (usageBucketRow, error) {
	bucket, err := loadUsageBucket(tx, input.TenantID, input.ModuleCode, input.PeriodKey, input.Metric)
	if err == nil {
		return bucket, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return usageBucketRow{}, err
	}
	bucket = usageBucketRow{TenantID: input.TenantID, ModuleCode: input.ModuleCode, PeriodKey: usageBucketPeriodKey(input.Metric, input.PeriodKey), Metric: input.Metric}
	if err := tx.Create(&bucket).Error; err != nil {
		return usageBucketRow{}, err
	}
	return bucket, nil
}

func loadUsageBucket(tx *gorm.DB, tenantID, moduleCode, periodKey, metric string) (usageBucketRow, error) {
	var bucket usageBucketRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", tenantID, moduleCode, usageBucketPeriodKey(metric, periodKey), metric).Take(&bucket).Error
	return bucket, err
}

func usageLimit(entitlement tenantEntitlementRow, metric string) (*int64, error) {
	limits, err := unmarshalLimits(entitlement.LimitsJSON)
	if err != nil {
		return nil, err
	}
	for _, key := range usageMetricLimitKeys(metric) {
		if value, ok := limits[key]; ok {
			limit := int64(value)
			return &limit, nil
		}
	}
	return nil, nil
}

func sumPositiveStorageReservations(tx *gorm.DB, input ReserveUsageInput) (int64, error) {
	var total int64
	err := tx.Model(&usageEventRow{}).
		Where("tenant_id = ? AND module_code = ? AND metric = ? AND status = ? AND quantity > 0", input.TenantID, input.ModuleCode, input.Metric, string(UsageEventReserved)).
		Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error
	return total, err
}

func validateUsageReservation(input ReserveUsageInput, bucket usageBucketRow, limit *int64, reservedForQuota int64) error {
	if err := ValidateProjectedUsage(input.Metric, bucket.Committed, bucket.Reserved, input.Quantity); err != nil {
		var quota *UsageQuotaError
		if errors.As(err, &quota) {
			quota.TenantID = input.TenantID
			quota.ModuleCode = input.ModuleCode
			quota.Limit = limit
		}
		return err
	}
	projected, ok := addUsage(bucket.Committed, reservedForQuota)
	if !ok {
		return &UsageValidationError{Field: "usage"}
	}
	projected, ok = addUsage(projected, input.Quantity)
	if !ok {
		return &UsageValidationError{Field: "quantity"}
	}
	if limit != nil && *limit > 0 && input.Quantity > 0 && projected > *limit {
		return &UsageQuotaError{TenantID: input.TenantID, ModuleCode: input.ModuleCode, Metric: input.Metric, Limit: limit, CommittedUsage: bucket.Committed, ReservedUsage: reservedForQuota, Quantity: input.Quantity}
	}
	return nil
}

func usageEventFromRow(row usageEventRow) UsageEvent {
	metadata := map[string]string(nil)
	if row.Metadata != "" {
		_ = json.Unmarshal([]byte(row.Metadata), &metadata)
	}
	return UsageEvent{EventID: row.EventID, TenantID: row.TenantID, ModuleCode: row.ModuleCode, Metric: row.Metric, Quantity: row.Quantity, PeriodKey: row.PeriodKey, SourceType: row.SourceType, SourceID: row.SourceID, IdempotencyKey: row.IdempotencyKey, Status: UsageEventStatus(row.Status), OccurredAt: row.OccurredAt, StorageSnapshot: cloneUsageInt64Pointer(row.StorageSnapshot), StorageSnapshotAt: cloneUsageTimePointer(row.StorageSnapshotAt), ReversalOf: row.ReversalOf, Metadata: metadata, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func cloneUsageInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneUsageTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func redactedUsageMetadata(reason string) (string, error) {
	metadata := map[string]string{"reason": "redacted"}
	if strings.TrimSpace(reason) == "" {
		metadata["reason"] = ""
	}
	data, err := json.Marshal(metadata)
	return string(data), err
}

func negateUsage(value int64) (int64, bool) {
	if value == -1<<63 {
		return 0, false
	}
	return -value, true
}
