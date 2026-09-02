package listingsubscription

import (
	"fmt"
	"strings"
	"time"
)

const (
	usageReconciliationReleasePendingKey = "listingkit_api_release_pending"
	usageReconciliationAsyncJobKey       = "listingkit_async_job"
	usageReconciliationMirrorKey         = "listingkit_legacy_counter_mirror"
	usageReconciliationMirrorSettled     = "settled"
)

const usageMetricStorageBytesCurrent = "storage_bytes_current"
const (
	usageMetricListingKitGenerationsSucceeded = "listingkit_generations_succeeded"
	usageMetricProductImageJobsSucceeded      = "product_image_jobs_succeeded"
	usageMetricSheinDraftsSucceeded           = "shein_drafts_succeeded"
	usageMetricSheinPublishesSucceeded        = "shein_publishes_succeeded"
)

const usageStorageBucketPeriodKey = "__current__"

func usageBucketPeriodKey(metric, periodKey string) string {
	if metric == usageMetricStorageBytesCurrent {
		return usageStorageBucketPeriodKey
	}
	return periodKey
}

// UsageValidationError identifies an invalid ledger input field.
type UsageValidationError struct {
	Field string
}

func (e *UsageValidationError) Error() string {
	return fmt.Sprintf("usage ledger invalid input: %s", e.Field)
}

func (e *UsageValidationError) Unwrap() error {
	return ErrUsageInvalidInput
}

// UsageDuplicateIdentityError identifies an event identity already owned by a tenant.
type UsageDuplicateIdentityError struct {
	TenantID       string
	IdempotencyKey string
}

func (e *UsageDuplicateIdentityError) Error() string {
	return fmt.Sprintf("usage ledger duplicate identity: tenant=%q idempotency_key=%q", e.TenantID, e.IdempotencyKey)
}

func (e *UsageDuplicateIdentityError) Unwrap() error {
	return ErrUsageDuplicateIdentity
}

// UsageTransitionError identifies a rejected event lifecycle transition.
type UsageTransitionError struct {
	From UsageEventStatus
	To   UsageEventStatus
}

func (e *UsageTransitionError) Error() string {
	return fmt.Sprintf("usage ledger invalid transition: %q -> %q", e.From, e.To)
}

func (e *UsageTransitionError) Unwrap() error {
	return ErrUsageInvalidTransition
}

// UsageQuotaError carries the safe usage context needed to diagnose a rejected reservation.
type UsageQuotaError struct {
	TenantID       string
	ModuleCode     string
	Metric         string
	Limit          *int64
	CommittedUsage int64
	ReservedUsage  int64
	Quantity       int64
}

func (e *UsageQuotaError) Error() string {
	limit := "none"
	if e.Limit != nil {
		limit = fmt.Sprintf("%d", *e.Limit)
	}
	return fmt.Sprintf("usage ledger quota exceeded: tenant=%q module=%q metric=%q limit=%s committed=%d reserved=%d quantity=%d", e.TenantID, e.ModuleCode, e.Metric, limit, e.CommittedUsage, e.ReservedUsage, e.Quantity)
}

func (e *UsageQuotaError) Unwrap() error {
	return ErrUsageQuotaExceeded
}

// NormalizeAndValidateReserveUsageInput returns a normalized, independently-owned reservation input.
func NormalizeAndValidateReserveUsageInput(input ReserveUsageInput) (ReserveUsageInput, error) {
	input = normalizeReserveUsageInput(input)
	if input.TenantID == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "tenant_id"}
	}
	if input.ModuleCode == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "module_code"}
	}
	if input.Metric == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "metric"}
	}
	if !isKnownUsageMetric(input.Metric) {
		return ReserveUsageInput{}, &UsageValidationError{Field: "metric"}
	}
	if !usageMetricModuleMatches(input.ModuleCode, input.Metric) {
		return ReserveUsageInput{}, &UsageValidationError{Field: "module_metric"}
	}
	if input.PeriodKey == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "period_key"}
	}
	if input.SourceType == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "source_type"}
	}
	if input.SourceID == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "source_id"}
	}
	if input.IdempotencyKey == "" {
		return ReserveUsageInput{}, &UsageValidationError{Field: "idempotency_key"}
	}
	if len(input.Metadata) != 0 {
		return ReserveUsageInput{}, ErrUsageOutboxUnsafeMetadata
	}
	if input.Quantity == 0 || (input.Metric != usageMetricStorageBytesCurrent && input.Quantity < 0) {
		return ReserveUsageInput{}, &UsageValidationError{Field: "quantity"}
	}
	if isUsageCountMetric(input.Metric) && input.Metric != usageMetricProductImageJobsSucceeded && input.Quantity != 1 {
		return ReserveUsageInput{}, &UsageValidationError{Field: "quantity"}
	}
	return input, nil
}

func isUsageCountMetric(metric string) bool {
	return metric == usageMetricListingKitGenerationsSucceeded || metric == usageMetricProductImageJobsSucceeded || metric == usageMetricSheinDraftsSucceeded || metric == usageMetricSheinPublishesSucceeded
}

func isKnownUsageMetric(metric string) bool {
	return metric == usageMetricStorageBytesCurrent || isUsageCountMetric(metric)
}

func usageMetricModuleMatches(moduleCode, metric string) bool {
	if metric == usageMetricStorageBytesCurrent {
		return moduleCode == ModuleOSSStorage
	}
	return moduleCode == ModuleStudio
}

// unrepresentedLegacyUsage returns only legacy counter usage that is not
// already represented by explicitly mirrored durable ledger events.
func unrepresentedLegacyUsage(legacyUsage, mirrored int64) int64 {
	if legacyUsage <= 0 {
		return 0
	}
	if mirrored < 0 {
		mirrored = 0
	}
	if mirrored >= legacyUsage {
		return 0
	}
	return legacyUsage - mirrored
}

func defaultUsageLedgerReconciliationFilter(tenantID, sourceType, metric string) UsageLedgerReconciliationFilter {
	return UsageLedgerReconciliationFilter{
		TenantID: tenantID, SourceType: sourceType, Metric: metric,
		ReservedMetadataPredicates: []UsageLedgerMetadataPredicate{
			{Key: usageReconciliationReleasePendingKey, Value: "1"},
			{Key: usageReconciliationAsyncJobKey, Value: "1"},
		},
		CommittedMetadataKey: usageReconciliationMirrorKey, CommittedSettledValue: usageReconciliationMirrorSettled,
	}
}

func usageEventMatchesReconciliationFilter(event UsageEvent, filter UsageLedgerReconciliationFilter) bool {
	if event.TenantID != strings.TrimSpace(filter.TenantID) || event.Metric != strings.TrimSpace(filter.Metric) || !containsUsageReconciliationValue(filter.SourceTypes, filter.SourceType, event.SourceType) {
		return false
	}
	if event.Status == UsageEventReserved {
		if containsUsageReconciliationValue(filter.ReservedSourceTypes, "", event.SourceType) {
			return true
		}
		for _, predicate := range filter.ReservedMetadataPredicates {
			if event.Metadata[strings.TrimSpace(predicate.Key)] == predicate.Value {
				return true
			}
		}
		return false
	}
	if event.Status == UsageEventCommitted {
		key := strings.TrimSpace(filter.CommittedMetadataKey)
		return key != "" && event.Metadata[key] != strings.TrimSpace(filter.CommittedSettledValue)
	}
	if event.Status == UsageEventReleased {
		for _, predicate := range filter.ReleasedMetadataPredicates {
			if event.Metadata[strings.TrimSpace(predicate.Key)] == predicate.Value {
				return true
			}
		}
	}
	return false
}

func containsUsageReconciliationValue(values []string, fallback, target string) bool {
	target = strings.TrimSpace(target)
	if len(values) == 0 {
		return target == strings.TrimSpace(fallback)
	}
	for _, value := range values {
		if target == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

func canonicalUsagePeriodKey(metric, supplied string, occurredAt time.Time) (string, error) {
	if metric == usageMetricStorageBytesCurrent {
		return supplied, nil
	}
	canonical := occurredAt.UTC().Format("2006-01")
	if supplied != canonical {
		return "", &UsageValidationError{Field: "period_key"}
	}
	return canonical, nil
}

func usageMetricLimitKeys(metric string) []string {
	switch metric {
	case usageMetricStorageBytesCurrent:
		return []string{usageMetricStorageBytesCurrent, "storage_bytes"}
	case usageMetricListingKitGenerationsSucceeded:
		return []string{usageMetricListingKitGenerationsSucceeded}
	case usageMetricProductImageJobsSucceeded:
		return []string{usageMetricProductImageJobsSucceeded, "product_image_jobs"}
	case usageMetricSheinDraftsSucceeded:
		return []string{usageMetricSheinDraftsSucceeded}
	case usageMetricSheinPublishesSucceeded:
		return []string{usageMetricSheinPublishesSucceeded}
	default:
		return []string{metric}
	}
}

func usageOutboxUndelivered(status string) bool {
	return status == "reserved" || status == "pending" || status == "cancelled"
}

// ValidateProjectedUsage rejects storage deltas that would take usage below zero.
func ValidateProjectedUsage(metric string, committedUsage, reservedUsage, quantity int64) error {
	if metric != usageMetricStorageBytesCurrent || quantity >= 0 {
		return nil
	}
	projected, ok := addUsage(committedUsage, reservedUsage)
	if !ok {
		return &UsageValidationError{Field: "usage"}
	}
	projected, ok = addUsage(projected, quantity)
	if !ok {
		return &UsageValidationError{Field: "quantity"}
	}
	if projected < 0 {
		return &UsageQuotaError{
			Metric:         metric,
			CommittedUsage: committedUsage,
			ReservedUsage:  reservedUsage,
			Quantity:       quantity,
		}
	}
	return nil
}

// ValidateUsageEventTransition permits only the irreversible ledger lifecycle edges.
func ValidateUsageEventTransition(from, to UsageEventStatus) error {
	if (from == UsageEventReserved && (to == UsageEventCommitted || to == UsageEventReleased)) ||
		(from == UsageEventCommitted && to == UsageEventReversed) {
		return nil
	}
	return &UsageTransitionError{From: from, To: to}
}

func normalizeReserveUsageInput(input ReserveUsageInput) ReserveUsageInput {
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.ModuleCode = strings.TrimSpace(input.ModuleCode)
	input.Metric = strings.TrimSpace(input.Metric)
	input.LegacyUsageMetric = strings.TrimSpace(input.LegacyUsageMetric)
	input.PeriodKey = strings.TrimSpace(input.PeriodKey)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.Metadata = cloneUsageMetadata(input.Metadata)
	return input
}

func usageEventMatchesReserveInput(event UsageEvent, input ReserveUsageInput) bool {
	return event.TenantID == input.TenantID &&
		event.ModuleCode == input.ModuleCode &&
		event.Metric == input.Metric &&
		event.Quantity == input.Quantity &&
		event.PeriodKey == input.PeriodKey &&
		event.SourceType == input.SourceType &&
		event.SourceID == input.SourceID
}

// usageReplayComparison preserves the persisted period only for legacy retries
// without an occurrence time. A caller that supplies an occurrence time has
// explicitly selected a billing period, which must match the event already
// claimed by its idempotency key.
func usageReplayComparison(input ReserveUsageInput, event UsageEvent) ReserveUsageInput {
	comparison := input
	if comparison.OccurredAt.IsZero() {
		comparison.PeriodKey = event.PeriodKey
	}
	return comparison
}

func validateUsageBucketTotals(metric string, committed, reserved int64) error {
	if metric != usageMetricStorageBytesCurrent && (committed < 0 || reserved < 0) {
		return &UsageValidationError{Field: "usage"}
	}
	total, ok := addUsage(committed, reserved)
	if !ok || total < 0 {
		return &UsageValidationError{Field: "usage"}
	}
	return nil
}

func cloneUsageMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func addUsage(left, right int64) (int64, bool) {
	const maxInt64 = 1<<63 - 1
	const minInt64 = -1 << 63
	if (right > 0 && left > maxInt64-right) || (right < 0 && left < minInt64-right) {
		return 0, false
	}
	return left + right, true
}
