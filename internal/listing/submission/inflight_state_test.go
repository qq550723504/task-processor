package submission

import (
	"testing"
	"time"
)

func TestBeginInFlightStateIncrementsAttemptAndStoresLease(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 20, 0, 0, 0, time.UTC)
	got := BeginInFlightState(InFlightState{AttemptCount: 2}, "publish", "req-1", "validate", startedAt, 15*time.Minute)

	if got.AttemptCount != 3 {
		t.Fatalf("attempt count = %d, want 3", got.AttemptCount)
	}
	if got.CurrentAction != "publish" || got.CurrentPhase != "validate" || got.CurrentRequestID != "req-1" {
		t.Fatalf("current in-flight state = %+v", got)
	}
	if got.InFlightStartedAt == nil || !got.InFlightStartedAt.Equal(startedAt) {
		t.Fatalf("in-flight started at = %+v, want %v", got.InFlightStartedAt, startedAt)
	}
	if got.LeaseExpiresAt == nil || !got.LeaseExpiresAt.Equal(startedAt.Add(15*time.Minute)) {
		t.Fatalf("lease expires at = %+v, want %v", got.LeaseExpiresAt, startedAt.Add(15*time.Minute))
	}
}

func TestAdvanceInFlightStatePreservesAttemptAndStart(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 14, 19, 50, 0, 0, time.UTC)
	now := startedAt.Add(10 * time.Minute)
	got := AdvanceInFlightState(InFlightState{
		AttemptCount:      4,
		CurrentAction:     "publish",
		CurrentPhase:      "validate",
		CurrentRequestID:  "req-1",
		InFlightStartedAt: &startedAt,
	}, "publish", "req-1", "submit_remote", now, 15*time.Minute)

	if got.AttemptCount != 4 {
		t.Fatalf("attempt count = %d, want 4", got.AttemptCount)
	}
	if got.InFlightStartedAt == nil || !got.InFlightStartedAt.Equal(startedAt) {
		t.Fatalf("in-flight started at = %+v, want %v", got.InFlightStartedAt, startedAt)
	}
	if got.CurrentPhase != "submit_remote" {
		t.Fatalf("current phase = %q, want submit_remote", got.CurrentPhase)
	}
	if got.LeaseExpiresAt == nil || !got.LeaseExpiresAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("lease expires at = %+v, want %v", got.LeaseExpiresAt, now.Add(15*time.Minute))
	}
}

func TestShouldClearInFlight(t *testing.T) {
	t.Parallel()

	if !ShouldClearInFlight("publish", "req-1", "publish", "req-1") {
		t.Fatal("expected matching action/request to clear in-flight state")
	}
	if ShouldClearInFlight("publish", "req-1", "save_draft", "req-1") {
		t.Fatal("mismatched action should not clear in-flight state")
	}
	if ShouldClearInFlight("publish", "req-1", "publish", "req-2") {
		t.Fatal("mismatched request should not clear in-flight state")
	}
}
