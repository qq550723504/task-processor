package submission

import (
	"errors"
	"testing"
	"time"
)

func TestResolveAttemptFinalizeStateUsesResolvedResultState(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 14, 21, 0, 0, 0, time.UTC)
	state := ResolveAttemptFinalizeState("publish", &ResponseOutcome{Success: true}, nil, finishedAt)

	if state.Status != "success" || state.ErrorMessage != "" {
		t.Fatalf("state = %+v, want success with empty error", state)
	}
	if !state.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finishedAt = %v, want %v", state.FinishedAt, finishedAt)
	}
}

func TestResolveAttemptFinalizeStateKeepsFailureError(t *testing.T) {
	t.Parallel()

	state := ResolveAttemptFinalizeState("publish", &ResponseOutcome{Success: true}, errors.New("boom"), time.Now())
	if state.Status != "failed" || state.ErrorMessage != "boom" {
		t.Fatalf("state = %+v, want failed/boom", state)
	}
}

func TestResolveAttemptFailureFinalizeStateSetsFailedStatus(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 14, 21, 5, 0, 0, time.UTC)
	state := ResolveAttemptFailureFinalizeState("prepare_product", errors.New("submit failed"), finishedAt)

	if state.Status != "failed" || state.ErrorMessage != "submit failed" {
		t.Fatalf("state = %+v, want failed/submit failed", state)
	}
	if !state.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finishedAt = %v, want %v", state.FinishedAt, finishedAt)
	}
}
