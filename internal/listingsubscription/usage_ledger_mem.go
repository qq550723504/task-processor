package listingsubscription

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewMemUsageLedger creates a deterministic, mutex-protected UsageLedger for
// service and handler tests. Its state is deliberately separate from legacy
// aggregate usage counters so it follows the durable ledger's reservation
// semantics.
func NewMemUsageLedger(repo *MemRepository) UsageLedger {
	return &memUsageLedger{
		repo:               repo,
		eventsByID:         map[string]memUsageEvent{},
		eventIDByIdentity:  map[string]string{},
		buckets:            map[string]memUsageBucket{},
		outboxByEventID:    map[string]UsageOutboxItem{},
		reversalIDBySource: map[string]string{},
		nextOutboxID:       1,
	}
}

type memUsageLedger struct {
	mu                 sync.Mutex
	repo               *MemRepository
	eventsByID         map[string]memUsageEvent
	eventIDByIdentity  map[string]string
	buckets            map[string]memUsageBucket
	outboxByEventID    map[string]UsageOutboxItem
	reversalIDBySource map[string]string
	nextOutboxID       int64
}

type memUsageEvent struct {
	event     UsageEvent
	periodKey string
}

type memUsageBucket struct {
	committed int64
	reserved  int64
}

func (l *memUsageLedger) Reserve(ctx context.Context, input ReserveUsageInput) (ReserveUsageResult, error) {
	_ = ctx
	input, err := NormalizeAndValidateReserveUsageInput(input)
	if err != nil {
		return ReserveUsageResult{}, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	identity := usageEventIdentityKey(input.TenantID, input.IdempotencyKey)
	if existingID, ok := l.eventIDByIdentity[identity]; ok {
		existing := l.eventsByID[existingID].event
		comparison := input
		if comparison.OccurredAt.IsZero() {
			comparison.PeriodKey = existing.PeriodKey
		}
		if !usageEventMatchesReserveInput(existing, comparison) {
			return ReserveUsageResult{}, &UsageDuplicateIdentityError{TenantID: input.TenantID, IdempotencyKey: input.IdempotencyKey}
		}
		return l.reserveResultForExisting(existingID)
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	if input.PeriodKey, err = canonicalUsagePeriodKey(input.Metric, input.PeriodKey, input.OccurredAt); err != nil {
		return ReserveUsageResult{}, err
	}

	entitlement, err := resolveMemUsageEntitlement(l.repo, input.TenantID, input.ModuleCode)
	if err != nil {
		return ReserveUsageResult{}, err
	}
	allowed, _ := evaluateEntitlement(entitlement, time.Now().UTC())
	if !allowed {
		return ReserveUsageResult{}, ErrSubscriptionRequired
	}
	limit := memUsageLimit(entitlement, input.Metric)
	bucketKey := memUsageBucketKey(input.TenantID, input.ModuleCode, input.PeriodKey, input.Metric)
	bucket := l.buckets[bucketKey]
	reservedForQuota := bucket.reserved
	if input.Metric == usageMetricStorageBytesCurrent && input.Quantity > 0 {
		reservedForQuota = 0
		for _, record := range l.eventsByID {
			if record.event.TenantID == input.TenantID && record.event.ModuleCode == input.ModuleCode && record.event.Metric == input.Metric && record.event.Status == UsageEventReserved && record.event.Quantity > 0 {
				reservedForQuota += record.event.Quantity
			}
		}
	}
	if err := validateMemUsageReservation(input, bucket, limit, reservedForQuota); err != nil {
		return ReserveUsageResult{}, err
	}
	reserved, ok := addUsage(bucket.reserved, input.Quantity)
	if !ok {
		return ReserveUsageResult{}, &UsageValidationError{Field: "quantity"}
	}
	bucket.reserved = reserved
	l.buckets[bucketKey] = bucket

	now := time.Now().UTC()
	event := UsageEvent{
		EventID: uuid.NewString(), TenantID: input.TenantID, ModuleCode: input.ModuleCode,
		Metric: input.Metric, Quantity: input.Quantity, PeriodKey: input.PeriodKey, SourceType: input.SourceType,
		SourceID: input.SourceID, IdempotencyKey: input.IdempotencyKey, Status: UsageEventReserved,
		OccurredAt: input.OccurredAt, Metadata: cloneUsageMetadata(input.Metadata), CreatedAt: now, UpdatedAt: now,
	}
	if input.Metric == usageMetricStorageBytesCurrent {
		snapshot := bucket.committed
		event.StorageSnapshot = &snapshot
	}
	l.eventsByID[event.EventID] = memUsageEvent{event: event, periodKey: input.PeriodKey}
	l.eventIDByIdentity[identity] = event.EventID
	l.addPendingOutbox(event.EventID, now)
	return ReserveUsageResult{Event: cloneMemUsageEvent(event), Limit: limit, CommittedUsage: bucket.committed, ReservedUsage: bucket.reserved}, nil
}

func (l *memUsageLedger) Commit(ctx context.Context, eventID string) (UsageEvent, error) {
	return l.transitionReservedEvent(ctx, eventID, UsageEventCommitted, "")
}

func (l *memUsageLedger) Release(ctx context.Context, eventID, reason string) (UsageEvent, error) {
	return l.transitionReservedEvent(ctx, eventID, UsageEventReleased, reason)
}

func (l *memUsageLedger) Reverse(ctx context.Context, eventID, idempotencyKey, reason string) (UsageEvent, error) {
	_ = ctx
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return UsageEvent{}, &UsageValidationError{Field: "idempotency_key"}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	source, ok := l.eventsByID[eventID]
	if !ok {
		return UsageEvent{}, usageEventNotFound(eventID)
	}
	if source.event.Status != UsageEventCommitted {
		return UsageEvent{}, ValidateUsageEventTransition(source.event.Status, UsageEventReversed)
	}
	sourceOutbox, sourceHasOutbox := l.outboxByEventID[source.event.EventID]
	sourceDelivered := !sourceHasOutbox || !usageOutboxUndelivered(sourceOutbox.Status)
	if sourceHasOutbox && sourceOutbox.Status == "failed" {
		return UsageEvent{}, ErrUsageReversalDeliveryUnresolved
	}
	laterDelivered := l.hasLaterDeliveredStorageSnapshot(source.event)
	if source.event.Metric != usageMetricStorageBytesCurrent && sourceDelivered {
		return UsageEvent{}, ErrUsageReversalProjectionUnsupported
	}
	if reversalID, ok := l.reversalIDBySource[source.event.EventID]; ok {
		return cloneMemUsageEvent(l.eventsByID[reversalID].event), nil
	}

	identity := usageEventIdentityKey(source.event.TenantID, idempotencyKey)
	if existingID, ok := l.eventIDByIdentity[identity]; ok {
		existing := l.eventsByID[existingID].event
		if existing.ReversalOf == source.event.EventID {
			return cloneMemUsageEvent(existing), nil
		}
		return UsageEvent{}, &UsageDuplicateIdentityError{TenantID: source.event.TenantID, IdempotencyKey: idempotencyKey}
	}

	quantity, ok := negateUsage(source.event.Quantity)
	if !ok {
		return UsageEvent{}, &UsageValidationError{Field: "quantity"}
	}
	bucketKey := memUsageBucketKey(source.event.TenantID, source.event.ModuleCode, source.periodKey, source.event.Metric)
	bucket, ok := l.buckets[bucketKey]
	if !ok {
		return UsageEvent{}, &UsageValidationError{Field: "usage"}
	}
	committed, ok := addUsage(bucket.committed, quantity)
	if !ok || validateUsageBucketTotals(source.event.Metric, committed, bucket.reserved) != nil {
		return UsageEvent{}, &UsageValidationError{Field: "usage"}
	}
	bucket.committed = committed
	l.buckets[bucketKey] = bucket

	now := time.Now().UTC()
	reversal := UsageEvent{
		EventID: uuid.NewString(), TenantID: source.event.TenantID, ModuleCode: source.event.ModuleCode,
		Metric: source.event.Metric, Quantity: quantity, PeriodKey: source.event.PeriodKey, SourceType: source.event.SourceType,
		SourceID: source.event.SourceID, IdempotencyKey: idempotencyKey, Status: UsageEventReversed,
		OccurredAt: now, ReversalOf: source.event.EventID, Metadata: redactedMemUsageMetadata(reason), CreatedAt: now, UpdatedAt: now,
	}
	reversalOutboxStatus := "cancelled"
	if (sourceDelivered || laterDelivered) && source.event.Metric == usageMetricStorageBytesCurrent {
		reversalOutboxStatus = "pending"
	} else if sourceHasOutbox && sourceOutbox.Status != "cancelled" {
		sourceOutbox.Status = "cancelled"
		sourceOutbox.UpdatedAt = now
		l.outboxByEventID[source.event.EventID] = sourceOutbox
	}
	l.addPendingOutboxWithStatus(reversal.EventID, now, reversalOutboxStatus)
	if source.event.Metric == usageMetricStorageBytesCurrent {
		snapshot := bucket.committed
		reversal.StorageSnapshot = &snapshot
		snapshotAt := now
		reversal.StorageSnapshotAt = &snapshotAt
	}
	l.eventsByID[reversal.EventID] = memUsageEvent{event: reversal, periodKey: source.periodKey}
	l.eventIDByIdentity[identity] = reversal.EventID
	l.reversalIDBySource[source.event.EventID] = reversal.EventID
	return cloneMemUsageEvent(reversal), nil
}

func (l *memUsageLedger) hasLaterDeliveredStorageSnapshot(source UsageEvent) bool {
	for id, record := range l.eventsByID {
		if id == source.EventID || record.event.TenantID != source.TenantID || record.event.ModuleCode != source.ModuleCode || record.event.Metric != usageMetricStorageBytesCurrent || (record.event.Status != UsageEventCommitted && record.event.Status != UsageEventReversed) {
			continue
		}
		item, ok := l.outboxByEventID[id]
		if !ok || usageOutboxUndelivered(item.Status) || item.Status == "failed" {
			continue
		}
		if source.StorageSnapshotAt != nil && record.event.StorageSnapshotAt != nil {
			if record.event.StorageSnapshotAt.After(*source.StorageSnapshotAt) {
				return true
			}
		} else if record.event.CreatedAt.After(source.CreatedAt) {
			return true
		}
	}
	return false
}

func (l *memUsageLedger) Get(ctx context.Context, tenantID, idempotencyKey string) (*UsageEvent, error) {
	_ = ctx
	l.mu.Lock()
	defer l.mu.Unlock()

	eventID, ok := l.eventIDByIdentity[usageEventIdentityKey(strings.TrimSpace(tenantID), strings.TrimSpace(idempotencyKey))]
	if !ok {
		return nil, usageEventNotFound("")
	}
	event := cloneMemUsageEvent(l.eventsByID[eventID].event)
	return &event, nil
}

func (l *memUsageLedger) ListPendingOutbox(ctx context.Context, limit int) ([]UsageOutboxItem, error) {
	_ = ctx
	if limit <= 0 {
		return []UsageOutboxItem{}, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UTC()
	items := make([]UsageOutboxItem, 0, len(l.outboxByEventID))
	for _, item := range l.outboxByEventID {
		if item.Status == "pending" {
			if item.NextAttemptAt != nil && item.NextAttemptAt.After(now) {
				continue
			}
			items = append(items, cloneMemUsageOutboxItem(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		leftAt, rightAt := left.CreatedAt, right.CreatedAt
		if left.NextAttemptAt != nil {
			leftAt = *left.NextAttemptAt
		}
		if right.NextAttemptAt != nil {
			rightAt = *right.NextAttemptAt
		}
		if leftAt.Equal(rightAt) {
			return left.ID < right.ID
		}
		return leftAt.Before(rightAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (l *memUsageLedger) transitionReservedEvent(ctx context.Context, eventID string, target UsageEventStatus, reason string) (UsageEvent, error) {
	_ = ctx
	l.mu.Lock()
	defer l.mu.Unlock()

	record, ok := l.eventsByID[eventID]
	if !ok {
		return UsageEvent{}, usageEventNotFound(eventID)
	}
	if record.event.Status == target {
		return cloneMemUsageEvent(record.event), nil
	}
	if record.event.Status != UsageEventReserved {
		return UsageEvent{}, ValidateUsageEventTransition(record.event.Status, target)
	}
	bucketKey := memUsageBucketKey(record.event.TenantID, record.event.ModuleCode, record.periodKey, record.event.Metric)
	bucket, ok := l.buckets[bucketKey]
	if !ok {
		return UsageEvent{}, &UsageValidationError{Field: "usage"}
	}
	reserved, ok := addUsage(bucket.reserved, -record.event.Quantity)
	if !ok {
		return UsageEvent{}, &UsageValidationError{Field: "usage"}
	}
	bucket.reserved = reserved
	if target == UsageEventCommitted {
		committed, ok := addUsage(bucket.committed, record.event.Quantity)
		if !ok {
			return UsageEvent{}, &UsageValidationError{Field: "quantity"}
		}
		bucket.committed = committed
	}
	if err := validateUsageBucketTotals(record.event.Metric, bucket.committed, bucket.reserved); err != nil {
		return UsageEvent{}, err
	}
	l.buckets[bucketKey] = bucket

	record.event.Status = target
	record.event.UpdatedAt = time.Now().UTC()
	if target == UsageEventReleased {
		payload := redactedMemUsageMetadata(reason)
		_, _ = l.repo.CreateAuditLog(ctx, AuditLog{TenantID: record.event.TenantID, ModuleCode: record.event.ModuleCode, Action: "usage_released", Payload: fmt.Sprintf(`{"reason":%q}`, payload["reason"])})
		if item, ok := l.outboxByEventID[eventID]; ok {
			item.Status = "cancelled"
			l.outboxByEventID[eventID] = item
		}
	} else if target == UsageEventCommitted {
		if item, ok := l.outboxByEventID[eventID]; ok && item.Status == "reserved" {
			item.Status = "pending"
			l.outboxByEventID[eventID] = item
		}
	}
	if record.event.Metric == usageMetricStorageBytesCurrent {
		snapshot := bucket.committed
		record.event.StorageSnapshot = &snapshot
		snapshotAt := time.Now().UTC()
		record.event.StorageSnapshotAt = &snapshotAt
	}
	l.eventsByID[eventID] = record
	return cloneMemUsageEvent(record.event), nil
}

func (l *memUsageLedger) reserveResultForExisting(eventID string) (ReserveUsageResult, error) {
	record, ok := l.eventsByID[eventID]
	if !ok {
		return ReserveUsageResult{}, usageEventNotFound(eventID)
	}
	bucket := l.buckets[memUsageBucketKey(record.event.TenantID, record.event.ModuleCode, record.periodKey, record.event.Metric)]
	entitlement, err := resolveMemUsageEntitlement(l.repo, record.event.TenantID, record.event.ModuleCode)
	if err != nil {
		return ReserveUsageResult{}, err
	}
	event := cloneMemUsageEvent(record.event)
	if record.event.Metric == usageMetricStorageBytesCurrent && event.StorageSnapshot == nil && record.event.Status == UsageEventReserved {
		snapshot := bucket.committed
		event.StorageSnapshot = &snapshot
	}
	return ReserveUsageResult{Event: event, Existing: true, CommittedUsage: bucket.committed, ReservedUsage: bucket.reserved, Limit: memUsageLimit(entitlement, record.event.Metric)}, nil
}

func (l *memUsageLedger) addPendingOutbox(eventID string, now time.Time) {
	l.addPendingOutboxWithStatus(eventID, now, "reserved")
}

func (l *memUsageLedger) addPendingOutboxWithStatus(eventID string, now time.Time, status string) {
	l.outboxByEventID[eventID] = UsageOutboxItem{ID: l.nextOutboxID, EventID: eventID, Destination: "openmeter", Status: status, CreatedAt: now, UpdatedAt: now}
	l.nextOutboxID++
}

func memUsageLimit(entitlement *Entitlement, metric string) *int64 {
	for _, key := range usageMetricLimitKeys(metric) {
		if value, ok := entitlement.Limits[key]; ok {
			limit := int64(value)
			return &limit
		}
	}
	return nil
}

func validateMemUsageReservation(input ReserveUsageInput, bucket memUsageBucket, limit *int64, reservedForQuota int64) error {
	if err := ValidateProjectedUsage(input.Metric, bucket.committed, bucket.reserved, input.Quantity); err != nil {
		if quota, ok := err.(*UsageQuotaError); ok {
			quota.TenantID = input.TenantID
			quota.ModuleCode = input.ModuleCode
			quota.Limit = limit
		}
		return err
	}
	projected, ok := addUsage(bucket.committed, reservedForQuota)
	if !ok {
		return &UsageValidationError{Field: "usage"}
	}
	projected, ok = addUsage(projected, input.Quantity)
	if !ok {
		return &UsageValidationError{Field: "quantity"}
	}
	if limit != nil && *limit > 0 && input.Quantity > 0 && projected > *limit {
		return &UsageQuotaError{TenantID: input.TenantID, ModuleCode: input.ModuleCode, Metric: input.Metric, Limit: limit, CommittedUsage: bucket.committed, ReservedUsage: reservedForQuota, Quantity: input.Quantity}
	}
	return nil
}

func memUsageBucketKey(tenantID, moduleCode, periodKey, metric string) string {
	return tenantID + "\x00" + moduleCode + "\x00" + usageBucketPeriodKey(metric, periodKey) + "\x00" + metric
}

func resolveMemUsageEntitlement(repo *MemRepository, tenantID, moduleCode string) (*Entitlement, error) {
	entitlement, err := repo.GetEntitlement(context.Background(), tenantID, moduleCode)
	if err == nil || moduleCode != ModuleOSSStorage || !errors.Is(err, ErrEntitlementNotFound) {
		return entitlement, err
	}
	studio, err := repo.GetEntitlement(context.Background(), tenantID, ModuleStudio)
	if err != nil {
		return nil, err
	}
	clone := *studio
	clone.ModuleCode = ModuleOSSStorage
	clone.Limits = nil
	return &clone, nil
}

func usageEventIdentityKey(tenantID, idempotencyKey string) string {
	return tenantID + "\x00" + idempotencyKey
}

func usageEventNotFound(eventID string) error {
	return fmt.Errorf("usage event not found: %q", eventID)
}

func cloneMemUsageEvent(event UsageEvent) UsageEvent {
	event.Metadata = cloneUsageMetadata(event.Metadata)
	if event.StorageSnapshot != nil {
		value := *event.StorageSnapshot
		event.StorageSnapshot = &value
	}
	if event.StorageSnapshotAt != nil {
		value := *event.StorageSnapshotAt
		event.StorageSnapshotAt = &value
	}
	return event
}

func cloneMemUsageOutboxItem(item UsageOutboxItem) UsageOutboxItem {
	if item.NextAttemptAt != nil {
		nextAttemptAt := *item.NextAttemptAt
		item.NextAttemptAt = &nextAttemptAt
	}
	return item
}

func redactedMemUsageMetadata(reason string) map[string]string {
	metadata := map[string]string{"reason": "redacted"}
	if strings.TrimSpace(reason) == "" {
		metadata["reason"] = ""
	}
	return metadata
}
