package openmeter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

type pocContractWindow struct {
	OccurredAt time.Time
	From       time.Time
	To         time.Time
}

func TestPoCCountMetersAggregateCommittedSuccesses(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := pocContractTenant(fixture, "count-committed")
	subject := mustPoCSubject(t, tenantID)

	tests := []struct {
		name    string
		metric  Metric
		meterID string
		events  int
		want    string
	}{
		{name: "studio design jobs", metric: MetricStudioDesignJobsSucceeded, meterID: fixture.Meters[0].ID, events: 3, want: "3"},
		{name: "SHEIN drafts", metric: MetricSheinDraftsSucceeded, meterID: fixture.Meters[1].ID, events: 2, want: "2"},
	}

	for _, test := range tests {
		for index := 0; index < test.events; index++ {
			event := mustPoCUsageEvent(t, UsageFact{
				TenantID:   tenantID,
				Metric:     test.metric,
				Quantity:   "1",
				SourceType: "committed_success",
				SourceID:   fmt.Sprintf("poc-%s-count-committed-%s-%d", fixture.Environment.RunID, test.metric, index),
				Revision:   "committed",
				OccurredAt: window.OccurredAt.Add(time.Duration(index) * time.Second),
			})
			mustPoCIngest(t, client, event)
		}
		waitForPoCUsage(t, client, test.meterID, subject, window.From, window.To, test.want, 1)
	}
}

func TestPoCDuplicateSourceAndIDCountsOnce(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := pocContractTenant(fixture, "duplicate-source-id")
	event := mustPoCUsageEvent(t, UsageFact{
		TenantID:   tenantID,
		Metric:     MetricStudioDesignJobsSucceeded,
		Quantity:   "1",
		SourceType: "design_job",
		SourceID:   "poc-" + fixture.Environment.RunID + "-duplicate-source-id",
		Revision:   "committed",
		OccurredAt: window.OccurredAt,
	})

	for attempt := 0; attempt < 10; attempt++ {
		mustPoCIngest(t, client, event)
	}
	waitForPoCUsage(t, client, fixture.Meters[0].ID, mustPoCSubject(t, tenantID), window.From, window.To, "1", 1)
}

func TestPoCSameIDDifferentSourceCountsSeparately(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantID := pocContractTenant(fixture, "same-id-different-source")
	event := mustPoCUsageEvent(t, UsageFact{
		TenantID:   tenantID,
		Metric:     MetricStudioDesignJobsSucceeded,
		Quantity:   "1",
		SourceType: "design_job",
		SourceID:   "poc-" + fixture.Environment.RunID + "-same-id-different-source",
		Revision:   "committed",
		OccurredAt: window.OccurredAt,
	})
	variant := event
	variant.Source = usageEventSource + "/alternate"

	mustPoCIngest(t, client, event)
	// OpenMeter deduplicates CloudEvents by Source and ID together. The alternate
	// Source is submitted through the official SDK because production Client
	// validation intentionally forbids it: production retries must never change
	// Source, or the same business fact will be counted twice.
	if err := fixture.SDK.Events.IngestEvent(t.Context(), variant); err != nil {
		t.Fatalf("ingest same-ID event with alternate Source through official SDK: %v", err)
	}

	waitForPoCUsage(t, client, fixture.Meters[0].ID, mustPoCSubject(t, tenantID), window.From, window.To, "2", 1)
}

func TestPoCTenantSubjectsRemainIsolated(t *testing.T) {
	fixture, client := requirePoCContractClient(t)
	window := newPoCContractWindow(t)
	tenantA := pocContractTenant(fixture, "tenant-isolation-a")
	tenantB := pocContractTenant(fixture, "tenant-isolation-b")

	for _, input := range []struct {
		tenantID string
		count    int
		suffix   string
	}{
		{tenantID: tenantA, count: 2, suffix: "a"},
		{tenantID: tenantB, count: 4, suffix: "b"},
	} {
		for index := 0; index < input.count; index++ {
			mustPoCIngest(t, client, mustPoCUsageEvent(t, UsageFact{
				TenantID:   input.tenantID,
				Metric:     MetricStudioDesignJobsSucceeded,
				Quantity:   "1",
				SourceType: "design_job",
				SourceID:   fmt.Sprintf("poc-%s-tenant-isolation-%s-%d", fixture.Environment.RunID, input.suffix, index),
				Revision:   "committed",
				OccurredAt: window.OccurredAt.Add(time.Duration(index) * time.Second),
			}))
		}
	}

	waitForPoCUsage(t, client, fixture.Meters[0].ID, mustPoCSubject(t, tenantA), window.From, window.To, "2", 1)
	waitForPoCUsage(t, client, fixture.Meters[0].ID, mustPoCSubject(t, tenantB), window.From, window.To, "4", 1)
}

func TestPoCInvalidEventsNeverReachOpenMeter(t *testing.T) {
	environment, err := loadPoCEnvironment()
	if err != nil {
		t.Fatalf("load OpenMeter PoC environment: %v", err)
	}
	if !environment.Enabled {
		t.Skip("OpenMeter PoC is disabled; set OPENMETER_POC=1 to opt in")
	}
	assertInvalidUsageEventsMakeZeroRequests(t, environment.RunID)
}

func TestInvalidUsageEventsMakeZeroHTTPRequests(t *testing.T) {
	assertInvalidUsageEventsMakeZeroRequests(t, "pure")
}

func assertInvalidUsageEventsMakeZeroRequests(t *testing.T, runID string) {
	t.Helper()
	window := newPoCContractWindow(t)
	validFact := func() UsageFact {
		return UsageFact{
			TenantID:   "poc-" + runID + "-invalid-events",
			Metric:     MetricStudioDesignJobsSucceeded,
			Quantity:   "1",
			SourceType: "design_job",
			SourceID:   "poc-" + runID + "-invalid-events",
			Revision:   "committed",
			OccurredAt: window.OccurredAt,
		}
	}

	tests := []struct {
		name     string
		exercise func(*testing.T, *Client) error
	}{
		{
			name: "unknown metric",
			exercise: func(*testing.T, *Client) error {
				fact := validFact()
				fact.Metric = Metric("unknown")
				_, err := BuildUsageEvent(fact)
				return err
			},
		},
		{
			name: "empty tenant",
			exercise: func(*testing.T, *Client) error {
				fact := validFact()
				fact.TenantID = ""
				_, err := BuildUsageEvent(fact)
				return err
			},
		},
		{
			name: "negative storage",
			exercise: func(*testing.T, *Client) error {
				fact := validFact()
				fact.Metric = MetricStorageBytesCurrent
				fact.Quantity = "-1"
				_, err := BuildUsageEvent(fact)
				return err
			},
		},
		{
			name: "non-decimal storage",
			exercise: func(*testing.T, *Client) error {
				fact := validFact()
				fact.Metric = MetricStorageBytesCurrent
				fact.Quantity = "1.5"
				_, err := BuildUsageEvent(fact)
				return err
			},
		},
		{
			name: "mutated event type mismatch",
			exercise: func(t *testing.T, client *Client) error {
				event := mustPoCUsageEvent(t, validFact())
				event.Type = eventTypeForMetric(MetricStorageBytesCurrent)
				return client.Ingest(t.Context(), event)
			},
		},
		{
			name: "mutated data metric mismatch",
			exercise: func(t *testing.T, client *Client) error {
				event := mustPoCUsageEvent(t, validFact())
				data := event.Data.GetOrEmpty()
				data["metric"] = string(MetricSheinDraftsSucceeded)
				event.Data = openmeterapi.NullableValue(data)
				return client.Ingest(t.Context(), event)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var requests atomic.Int32
			httpClient := &http.Client{Transport: pocRoundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return nil, errors.New("invalid event reached HTTP transport")
			})}
			client, err := NewClient(Config{BaseURL: "http://openmeter.invalid/api/v3", HTTPClient: httpClient})
			if err != nil {
				t.Fatalf("construct validation client: %v", err)
			}

			if err := test.exercise(t, client); err == nil {
				t.Fatal("invalid event error = nil, want local validation error")
			}
			if got := requests.Load(); got != 0 {
				t.Fatalf("invalid event made %d HTTP requests, want 0", got)
			}
		})
	}
}

func requirePoCContractClient(t *testing.T) (*pocFixture, *Client) {
	t.Helper()
	fixture := requirePoCFixture(t)
	return fixture, &Client{sdk: fixture.SDK}
}

func newPoCContractWindow(t *testing.T) pocContractWindow {
	t.Helper()
	occurredAt := time.Now().UTC().Truncate(time.Second)
	window := pocContractWindow{
		OccurredAt: occurredAt,
		From:       occurredAt.Add(-time.Minute),
		To:         occurredAt.Add(time.Minute),
	}
	if window.OccurredAt.Location() != time.UTC || window.From.Location() != time.UTC || window.To.Location() != time.UTC {
		t.Fatal("PoC contract window must use explicit UTC timestamps")
	}
	return window
}

func pocContractTenant(fixture *pocFixture, suffix string) string {
	return fmt.Sprintf("poc-%s-task5-%s", fixture.Environment.RunID, suffix)
}

func mustPoCSubject(t *testing.T, tenantID string) string {
	t.Helper()
	subject, err := SubjectForTenant(tenantID)
	if err != nil {
		t.Fatalf("build PoC subject for tenant %q: %v", tenantID, err)
	}
	return subject
}

func mustPoCUsageEvent(t *testing.T, fact UsageFact) openmeterapi.EventInput {
	t.Helper()
	event, err := BuildUsageEvent(fact)
	if err != nil {
		t.Fatalf("build PoC usage event: %v", err)
	}
	return event
}

func mustPoCIngest(t *testing.T, client *Client, event openmeterapi.EventInput) {
	t.Helper()
	if err := client.Ingest(t.Context(), event); err != nil {
		t.Fatalf("ingest PoC usage event %q: %v", event.ID, err)
	}
}

func waitForPoCUsage(t *testing.T, client *Client, meterID, subject string, from, to time.Time, want string, consecutiveSamples int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), pocMeterPollTimeout)
	defer cancel()
	ticker := time.NewTicker(pocMeterPollInterval)
	defer ticker.Stop()

	lastResult := "no query completed"
	matched := 0
	for {
		got, err := client.QueryUsage(ctx, meterID, subject, from, to)
		if err == nil {
			lastResult = fmt.Sprintf("value %q", got)
			if got == want {
				matched++
				if matched >= consecutiveSamples {
					return
				}
			} else {
				matched = 0
			}
		} else {
			lastResult = "error: " + err.Error()
			matched = 0
		}

		select {
		case <-ctx.Done():
			t.Fatalf("OpenMeter usage for meter %q subject %q did not become exactly %q within %s; last result: %s", meterID, subject, want, pocMeterPollTimeout, lastResult)
		case <-ticker.C:
		}
	}
}

func waitForPoCEventStored(t *testing.T, sdk *openmeterapi.Client, event openmeterapi.EventInput) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), pocMeterPollTimeout)
	defer cancel()
	ticker := time.NewTicker(pocMeterPollInterval)
	defer ticker.Stop()

	lastResult := "no event lookup completed"
	for {
		page, err := sdk.Events.List(ctx, openmeterapi.IngestedEventListParams{
			Filter: &openmeterapi.IngestedEventFilter{
				ID:     &openmeterapi.StringFilter{Eq: &event.ID},
				Source: &openmeterapi.StringFilter{Eq: &event.Source},
			},
		})
		if err == nil {
			lastResult = fmt.Sprintf("%d matching events", len(page.Data))
			if len(page.Data) == 1 && !page.Data[0].StoredAt.IsZero() {
				if len(page.Data[0].ValidationErrors) != 0 {
					t.Fatalf("OpenMeter stored event %q with validation errors: %+v", event.ID, page.Data[0].ValidationErrors)
				}
				return
			}
		} else {
			lastResult = "error: " + err.Error()
		}

		select {
		case <-ctx.Done():
			t.Fatalf("OpenMeter event %q source %q was not visibly stored within %s; last result: %s", event.ID, event.Source, pocMeterPollTimeout, lastResult)
		case <-ticker.C:
		}
	}
}
