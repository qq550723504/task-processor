package openmeter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

const (
	usageEventSource     = "task-processor/listingkit"
	usageEventTypePrefix = "listingkit.usage."
)

// Metric identifies the catalog meter represented by a usage event.
type Metric string

const (
	MetricStudioDesignJobsSucceeded Metric = "studio_design_jobs_succeeded"
	MetricSheinDraftsSucceeded      Metric = "shein_drafts_succeeded"
	MetricStorageBytesCurrent       Metric = "storage_bytes_current"
)

// UsageFact contains the business facts from which a metering event is built.
type UsageFact struct {
	TenantID   string
	Metric     Metric
	Quantity   string
	SourceType string
	SourceID   string
	Revision   string
	OccurredAt time.Time
}

// BuildUsageEvent builds the stable CloudEvent submitted to OpenMeter.
func BuildUsageEvent(fact UsageFact) (openmeterapi.EventInput, error) {
	quantity, err := validateUsageFact(fact)
	if err != nil {
		return openmeterapi.EventInput{}, err
	}

	subject, err := SubjectForTenant(fact.TenantID)
	if err != nil {
		return openmeterapi.EventInput{}, err
	}
	specversion := "1.0"
	return openmeterapi.EventInput{
		ID:              usageEventID(fact),
		Source:          usageEventSource,
		Specversion:     &specversion,
		Type:            eventTypeForMetric(fact.Metric),
		Datacontenttype: openmeterapi.NullableValue("application/json"),
		Subject:         subject,
		Time:            openmeterapi.NullableValue(fact.OccurredAt),
		Data: openmeterapi.NullableValue(map[string]any{
			"metric":      string(fact.Metric),
			"quantity":    quantity,
			"source_type": fact.SourceType,
			"source_id":   fact.SourceID,
			"revision":    fact.Revision,
		}),
	}, nil
}

// ValidateUsageEvent verifies that an event conforms to the usage-event contract.
func ValidateUsageEvent(event openmeterapi.EventInput) error {
	if event.Source != usageEventSource {
		return fmt.Errorf("usage event source must be %q", usageEventSource)
	}
	if event.ID == "" {
		return fmt.Errorf("usage event ID is required")
	}
	if event.Specversion == nil || *event.Specversion != "1.0" {
		return fmt.Errorf("usage event specversion must be 1.0")
	}
	if event.Datacontenttype.GetOrEmpty() != "application/json" {
		return fmt.Errorf("usage event datacontenttype must be application/json")
	}
	if event.Subject == "" {
		return fmt.Errorf("usage event subject is required")
	}
	if event.Time.GetOrEmpty().IsZero() || event.Time.GetOrEmpty().Location() != time.UTC {
		return fmt.Errorf("usage event time must be non-zero UTC")
	}

	data, err := event.Data.Get()
	if err != nil {
		return fmt.Errorf("usage event data is required: %w", err)
	}
	metricValue, ok := data["metric"].(string)
	if !ok {
		return fmt.Errorf("usage event data.metric must be a string")
	}
	metric := Metric(metricValue)
	if !isKnownMetric(metric) {
		return fmt.Errorf("unknown usage metric %q", metric)
	}
	if event.Type != eventTypeForMetric(metric) {
		return fmt.Errorf("usage event type %q does not match metric %q", event.Type, metric)
	}

	for key := range data {
		if key != "metric" && key != "quantity" && key != "source_type" && key != "source_id" && key != "revision" {
			return fmt.Errorf("usage event data contains disallowed field %q", key)
		}
	}
	quantity, ok := data["quantity"].(string)
	if !ok {
		return fmt.Errorf("usage event data.quantity must be a string")
	}
	if _, ok := data["source_type"].(string); !ok || data["source_type"] == "" {
		return fmt.Errorf("usage event data.source_type must be a non-empty string")
	}
	if _, ok := data["source_id"].(string); !ok || data["source_id"] == "" {
		return fmt.Errorf("usage event data.source_id must be a non-empty string")
	}
	if _, ok := data["revision"].(string); !ok || data["revision"] == "" {
		return fmt.Errorf("usage event data.revision must be a non-empty string")
	}
	_, err = validateMetricQuantity(metric, quantity)
	return err
}

// SubjectForTenant returns the CloudEvent subject for a tenant.
func SubjectForTenant(tenantID string) (string, error) {
	if tenantID == "" {
		return "", fmt.Errorf("tenant ID is required")
	}
	return "tenant/" + url.PathEscape(tenantID), nil
}

func validateUsageFact(fact UsageFact) (string, error) {
	if fact.TenantID == "" {
		return "", fmt.Errorf("tenant ID is required")
	}
	if !isKnownMetric(fact.Metric) {
		return "", fmt.Errorf("unknown usage metric %q", fact.Metric)
	}
	if fact.SourceType == "" || fact.SourceID == "" || fact.Revision == "" {
		return "", fmt.Errorf("usage fact source type, source ID, and revision are required")
	}
	if fact.OccurredAt.IsZero() || fact.OccurredAt.Location() != time.UTC {
		return "", fmt.Errorf("usage fact occurrence time must be non-zero UTC")
	}
	return validateMetricQuantity(fact.Metric, fact.Quantity)
}

func validateMetricQuantity(metric Metric, quantity string) (string, error) {
	if metric == MetricStudioDesignJobsSucceeded || metric == MetricSheinDraftsSucceeded {
		if quantity != "1" {
			return "", fmt.Errorf("count metric %q requires quantity 1", metric)
		}
		return quantity, nil
	}
	if quantity == "" {
		return "", fmt.Errorf("storage quantity is required")
	}
	for _, character := range quantity {
		if character < '0' || character > '9' {
			return "", fmt.Errorf("storage quantity %q must be a non-negative base-10 integer", quantity)
		}
	}
	normalized := strings.TrimLeft(quantity, "0")
	if normalized == "" {
		normalized = "0"
	}
	return normalized, nil
}

func eventTypeForMetric(metric Metric) string {
	return usageEventTypePrefix + string(metric)
}

func isKnownMetric(metric Metric) bool {
	return metric == MetricStudioDesignJobsSucceeded || metric == MetricSheinDraftsSucceeded || metric == MetricStorageBytesCurrent
}

func usageEventID(fact UsageFact) string {
	parts := []string{fact.TenantID, string(fact.Metric), fact.SourceType, fact.SourceID, fact.Revision}
	var identity strings.Builder
	for _, part := range parts {
		identity.WriteString(strconv.Itoa(len(part)))
		identity.WriteByte(':')
		identity.WriteString(part)
	}
	sum := sha256.Sum256([]byte(identity.String()))
	return "listingkit-usage-" + hex.EncodeToString(sum[:])
}
