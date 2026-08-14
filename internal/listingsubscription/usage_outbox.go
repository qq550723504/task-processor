package listingsubscription

import (
	"errors"
	"strings"
)

// BuildOpenMeterUsageOutboxPayload constructs the small, provider-safe fact a
// future OpenMeter worker may project asynchronously. It is deliberately pure:
// validation or later delivery failures cannot change the local ledger event.
func BuildOpenMeterUsageOutboxPayload(event UsageEvent) (OpenMeterUsageOutboxPayload, error) {
	if event.Status == UsageEventReversed && event.Metric != usageMetricStorageBytesCurrent {
		return OpenMeterUsageOutboxPayload{}, ErrUsageReversalProjectionUnsupported
	}
	if event.Status != UsageEventCommitted && !(event.Status == UsageEventReversed && event.Metric == usageMetricStorageBytesCurrent) {
		return OpenMeterUsageOutboxPayload{}, errors.New("usage outbox event must be committed")
	}
	if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.TenantID) == "" ||
		strings.TrimSpace(event.ModuleCode) == "" || strings.TrimSpace(event.Metric) == "" ||
		strings.TrimSpace(event.SourceType) == "" || strings.TrimSpace(event.SourceID) == "" || event.OccurredAt.IsZero() {
		return OpenMeterUsageOutboxPayload{}, errors.New("usage outbox event is incomplete")
	}
	if len(event.Metadata) != 0 && !(event.Status == UsageEventReversed && event.Metric == usageMetricStorageBytesCurrent && isRedactedReversalMetadata(event.Metadata)) {
		return OpenMeterUsageOutboxPayload{}, ErrUsageOutboxUnsafeMetadata
	}
	quantity := event.Quantity
	if event.Metric == usageMetricStorageBytesCurrent {
		if event.StorageSnapshot == nil || *event.StorageSnapshot < 0 {
			return OpenMeterUsageOutboxPayload{}, ErrUsageOutboxStorageSnapshotRequired
		}
		quantity = *event.StorageSnapshot
	}
	occurredAt := event.OccurredAt.UTC()
	if event.Metric == usageMetricStorageBytesCurrent && event.StorageSnapshotAt != nil {
		occurredAt = event.StorageSnapshotAt.UTC()
	}
	return OpenMeterUsageOutboxPayload{
		EventID:    event.EventID,
		TenantID:   event.TenantID,
		Metric:     event.Metric,
		Quantity:   quantity,
		OccurredAt: occurredAt,
		Metadata: map[string]string{
			"module_code": event.ModuleCode,
			"source_type": event.SourceType,
			"source_id":   event.SourceID,
		},
	}, nil
}

func isRedactedReversalMetadata(metadata map[string]string) bool {
	if len(metadata) != 1 {
		return false
	}
	reason, ok := metadata["reason"]
	return ok && (reason == "" || reason == "redacted")
}

