package listingsubscription

import (
	"errors"
	"strings"
)

// BuildOpenMeterUsageOutboxPayload constructs the small, provider-safe fact a
// future OpenMeter worker may project asynchronously. It is deliberately pure:
// validation or later delivery failures cannot change the local ledger event.
func BuildOpenMeterUsageOutboxPayload(event UsageEvent) (OpenMeterUsageOutboxPayload, error) {
	if event.Status != UsageEventCommitted {
		return OpenMeterUsageOutboxPayload{}, errors.New("usage outbox event must be committed")
	}
	if strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.TenantID) == "" ||
		strings.TrimSpace(event.ModuleCode) == "" || strings.TrimSpace(event.Metric) == "" ||
		strings.TrimSpace(event.SourceType) == "" || strings.TrimSpace(event.SourceID) == "" || event.OccurredAt.IsZero() {
		return OpenMeterUsageOutboxPayload{}, errors.New("usage outbox event is incomplete")
	}
	if len(event.Metadata) != 0 {
		return OpenMeterUsageOutboxPayload{}, ErrUsageOutboxUnsafeMetadata
	}
	return OpenMeterUsageOutboxPayload{
		EventID:    event.EventID,
		TenantID:   event.TenantID,
		Metric:     event.Metric,
		Quantity:   event.Quantity,
		OccurredAt: event.OccurredAt.UTC(),
		Metadata: map[string]string{
			"module_code": event.ModuleCode,
			"source_type": event.SourceType,
			"source_id":   event.SourceID,
		},
	}, nil
}
