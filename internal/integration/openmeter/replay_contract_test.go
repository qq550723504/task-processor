package openmeter

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	openmeterapi "github.com/openmeterio/openmeter/api/v3/client"
)

var pocReplayBaseTime = time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC)

func TestPoCReplaySeed(t *testing.T) {
	environment := requirePoCReplayPhase(t, "seed")
	fixture := requirePoCFixture(t)
	client := &Client{sdk: fixture.SDK}
	events, window := pocReplayEvents(t, environment.RunID)

	for _, event := range events[:3] {
		mustPoCIngest(t, client, event)
	}
	waitForPoCUsage(t, client, fixture.Meters[0].ID, events[0].Subject, window.From, window.To, "3", 1)
}

func TestPoCUnavailableClassifiesFailureAsRetryable(t *testing.T) {
	environment := requirePoCReplayPhase(t, "unavailable")
	events, _ := pocReplayEvents(t, environment.RunID)
	event := events[3]
	wantIdentity := pocReplayIdentityOf(t, event)

	client, err := NewClient(Config{
		BaseURL: environment.BaseURL,
		APIKey:  environment.APIKey,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	})
	if err != nil {
		t.Fatalf("construct unavailable-phase OpenMeter client: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	err = client.Ingest(ctx, event)
	if err == nil {
		t.Fatal("ingest while the OpenMeter API service is stopped returned nil, want retryable error")
	}
	if got := ClassifyError(err); got != FailureRetryable {
		t.Fatalf("ClassifyError(ingest while stopped) = %q, want %q; error: %v", got, FailureRetryable, err)
	}
	if got := pocReplayIdentityOf(t, event); !reflect.DeepEqual(got, wantIdentity) {
		t.Fatalf("unavailable ingest changed event identity: got %+v, want %+v", got, wantIdentity)
	}
}

func TestPoCReplayAfterRecoveryConvergesExactly(t *testing.T) {
	environment := requirePoCReplayPhase(t, "replay")
	fixture := requirePoCFixture(t)
	client := &Client{sdk: fixture.SDK}
	events, window := pocReplayEvents(t, environment.RunID)

	for _, event := range events {
		mustPoCIngest(t, client, event)
	}
	for duplicate := 0; duplicate < 10; duplicate++ {
		mustPoCIngest(t, client, events[3])
	}
	waitForPoCUsage(t, client, fixture.Meters[0].ID, events[0].Subject, window.From, window.To, "4", 4)
}

type pocReplayWindow struct {
	From time.Time
	To   time.Time
}

type pocReplayIdentity struct {
	ID         string
	Source     string
	Subject    string
	Metric     string
	SourceType string
	SourceID   string
	Revision   string
}

func requirePoCReplayPhase(t *testing.T, want string) pocEnvironment {
	t.Helper()
	environment, err := loadPoCEnvironment()
	if err != nil {
		t.Fatalf("load OpenMeter PoC environment: %v", err)
	}
	if !environment.Enabled {
		t.Skip("OpenMeter PoC is disabled; set OPENMETER_POC=1 to opt in")
	}
	if environment.Phase != want {
		t.Skipf("OpenMeter PoC phase is %q; replay test requires %q", environment.Phase, want)
	}
	return environment
}

func pocReplayEvents(t *testing.T, runID string) ([]openmeterapi.EventInput, pocReplayWindow) {
	t.Helper()
	tenantID := fmt.Sprintf("poc-%s-task7-replay", runID)
	events := make([]openmeterapi.EventInput, 0, 4)
	for index := 1; index <= 4; index++ {
		events = append(events, mustPoCUsageEvent(t, UsageFact{
			TenantID:   tenantID,
			Metric:     MetricStudioDesignJobsSucceeded,
			Quantity:   "1",
			SourceType: "design_job",
			SourceID:   fmt.Sprintf("poc-%s-replay-success-%d", runID, index),
			Revision:   "committed",
			OccurredAt: pocReplayBaseTime.Add(time.Duration(index) * time.Second),
		}))
	}
	return events, pocReplayWindow{
		From: pocReplayBaseTime.Add(-time.Minute),
		To:   pocReplayBaseTime.Add(time.Minute),
	}
}

func pocReplayIdentityOf(t *testing.T, event openmeterapi.EventInput) pocReplayIdentity {
	t.Helper()
	data, err := event.Data.Get()
	if err != nil {
		t.Fatalf("read replay event %q data: %v", event.ID, err)
	}
	return pocReplayIdentity{
		ID:         event.ID,
		Source:     event.Source,
		Subject:    event.Subject,
		Metric:     fmt.Sprint(data["metric"]),
		SourceType: fmt.Sprint(data["source_type"]),
		SourceID:   fmt.Sprint(data["source_id"]),
		Revision:   fmt.Sprint(data["revision"]),
	}
}
