package submission

import (
	"testing"
	"time"
)

func TestNewRemoteRefreshExecutionStateCarriesCompletionSupplierAndStartedAt(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 26, 9, 30, 0, 0, time.UTC)
	state := NewRemoteRefreshExecutionState("completion", "SUP-1", startedAt)

	if state.Completion != "completion" {
		t.Fatalf("Completion = %q, want completion", state.Completion)
	}
	if state.SupplierCode != "SUP-1" {
		t.Fatalf("SupplierCode = %q, want SUP-1", state.SupplierCode)
	}
	if !state.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want %v", state.StartedAt, startedAt)
	}
}
