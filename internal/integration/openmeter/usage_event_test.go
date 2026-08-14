package openmeter

import (
	"reflect"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

func TestBuildUsageEventCreatesStableCloudEvent(t *testing.T) {
	fact := validUsageFact()

	first, err := BuildUsageEvent(fact)
	if err != nil {
		t.Fatalf("BuildUsageEvent() error = %v", err)
	}
	second, err := BuildUsageEvent(fact)
	if err != nil {
		t.Fatalf("BuildUsageEvent() second error = %v", err)
	}

	if first.Source != "task-processor/listingkit" {
		t.Errorf("Source = %q, want task-processor/listingkit", first.Source)
	}
	if first.Specversion == nil || *first.Specversion != "1.0" {
		t.Errorf("Specversion = %v, want 1.0", first.Specversion)
	}
	if got := first.Datacontenttype.GetOrEmpty(); got != "application/json" {
		t.Errorf("Datacontenttype = %q, want application/json", got)
	}
	if first.ID != second.ID || first.Subject != second.Subject || first.Type != second.Type ||
		first.Source != second.Source || !reflect.DeepEqual(first.Time, second.Time) || !reflect.DeepEqual(first.Data, second.Data) {
		t.Errorf("BuildUsageEvent() did not create stable events: first=%+v second=%+v", first, second)
	}
}

func TestBuildUsageEventMapsEachMetricToDistinctEventType(t *testing.T) {
	tests := []struct {
		name   string
		metric Metric
		want   string
	}{
		{"studio design jobs", MetricStudioDesignJobsSucceeded, "listingkit.usage.studio_design_jobs_succeeded"},
		{"product image jobs", MetricProductImageJobsSucceeded, "listingkit.usage.product_image_jobs_succeeded"},
		{"shein drafts", MetricSheinDraftsSucceeded, "listingkit.usage.shein_drafts_succeeded"},
		{"shein publishes", MetricSheinPublishesSucceeded, "listingkit.usage.shein_publishes_succeeded"},
		{"storage bytes", MetricStorageBytesCurrent, "listingkit.usage.storage_bytes_current"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fact := validUsageFact()
			fact.Metric = tt.metric
			if tt.metric == MetricStorageBytesCurrent {
				fact.Quantity = "42"
			}
			event, err := BuildUsageEvent(fact)
			if err != nil {
				t.Fatalf("BuildUsageEvent() error = %v", err)
			}
			if event.Type != tt.want {
				t.Errorf("Type = %q, want %q", event.Type, tt.want)
			}
		})
	}
}

func TestBuildUsageEventRejectsInvalidFacts(t *testing.T) {
	tests := []struct {
		name string
		fact UsageFact
	}{
		{"empty tenant", func() UsageFact { f := validUsageFact(); f.TenantID = ""; return f }()},
		{"unknown metric", func() UsageFact { f := validUsageFact(); f.Metric = "unknown"; return f }()},
		{"empty source type", func() UsageFact { f := validUsageFact(); f.SourceType = ""; return f }()},
		{"empty source ID", func() UsageFact { f := validUsageFact(); f.SourceID = ""; return f }()},
		{"empty revision", func() UsageFact { f := validUsageFact(); f.Revision = ""; return f }()},
		{"non-UTC time", func() UsageFact {
			f := validUsageFact()
			f.OccurredAt = time.Date(2026, 8, 13, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
			return f
		}()},
		{"zero time", func() UsageFact { f := validUsageFact(); f.OccurredAt = time.Time{}; return f }()},
		{"count quantity other than one", func() UsageFact { f := validUsageFact(); f.Quantity = "2"; return f }()},
		{"negative storage", func() UsageFact {
			f := validUsageFact()
			f.Metric = MetricStorageBytesCurrent
			f.Quantity = "-1"
			return f
		}()},
		{"fractional storage", func() UsageFact {
			f := validUsageFact()
			f.Metric = MetricStorageBytesCurrent
			f.Quantity = "1.5"
			return f
		}()},
		{"storage exponent notation", func() UsageFact {
			f := validUsageFact()
			f.Metric = MetricStorageBytesCurrent
			f.Quantity = "1e3"
			return f
		}()},
		{"non-decimal storage", func() UsageFact {
			f := validUsageFact()
			f.Metric = MetricStorageBytesCurrent
			f.Quantity = "0x10"
			return f
		}()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildUsageEvent(tt.fact); err == nil {
				t.Fatal("BuildUsageEvent() error = nil, want validation error")
			}
		})
	}
}

func TestValidateUsageEventRejectsMetricTypeMismatch(t *testing.T) {
	event, err := BuildUsageEvent(validUsageFact())
	if err != nil {
		t.Fatalf("BuildUsageEvent() error = %v", err)
	}
	event.Type = "listingkit.usage.storage_bytes_current"

	if err := ValidateUsageEvent(event); err == nil {
		t.Fatal("ValidateUsageEvent() error = nil, want metric/type mismatch error")
	}
}

func TestValidateUsageEventRejectsTamperedIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*openmeterapi.EventInput)
	}{
		{
			name: "subject",
			mutate: func(event *openmeterapi.EventInput) {
				event.Subject = "account:tenant-17"
			},
		},
		{
			name: "ID",
			mutate: func(event *openmeterapi.EventInput) {
				event.ID = "retry-unique-event-id"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := BuildUsageEvent(validUsageFact())
			if err != nil {
				t.Fatalf("BuildUsageEvent() error = %v", err)
			}
			tt.mutate(&event)

			if err := ValidateUsageEvent(event); err == nil {
				t.Fatal("ValidateUsageEvent() error = nil, want identity validation error")
			}
		})
	}
}

func TestBuildUsageEventDataContainsOnlyAllowlistedFields(t *testing.T) {
	event, err := BuildUsageEvent(validUsageFact())
	if err != nil {
		t.Fatalf("BuildUsageEvent() error = %v", err)
	}

	want := map[string]any{
		"metric":      "studio_design_jobs_succeeded",
		"quantity":    "1",
		"source_type": "design_job",
		"source_id":   "job-42",
		"revision":    "v3",
	}
	if got := event.Data.GetOrEmpty(); !reflect.DeepEqual(got, want) {
		t.Errorf("Data = %#v, want %#v", got, want)
	}
}

func TestBuildUsageEventNormalizesStorageQuantity(t *testing.T) {
	fact := validUsageFact()
	fact.Metric = MetricStorageBytesCurrent
	fact.Quantity = "00042"

	event, err := BuildUsageEvent(fact)
	if err != nil {
		t.Fatalf("BuildUsageEvent() error = %v", err)
	}
	if got := event.Data.GetOrEmpty()["quantity"]; got != "42" {
		t.Errorf("Data quantity = %q, want 42", got)
	}

	fact.Quantity = "000"
	event, err = BuildUsageEvent(fact)
	if err != nil {
		t.Fatalf("BuildUsageEvent() zero storage error = %v", err)
	}
	if got := event.Data.GetOrEmpty()["quantity"]; got != "0" {
		t.Errorf("Data quantity = %q, want 0", got)
	}
}

func validUsageFact() UsageFact {
	return UsageFact{
		TenantID:   "tenant-17",
		Metric:     MetricStudioDesignJobsSucceeded,
		Quantity:   "1",
		SourceType: "design_job",
		SourceID:   "job-42",
		Revision:   "v3",
		OccurredAt: time.Date(2026, 8, 13, 2, 0, 0, 0, time.UTC),
	}
}

