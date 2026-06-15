package submission

import (
	"testing"
	"time"
)

func TestResolveSubmitRequestIDPrefersIdempotencyKey(t *testing.T) {
	t.Parallel()

	got := ResolveSubmitRequestID(" idem-1 ", "req-1")
	if got != "idem-1" {
		t.Fatalf("ResolveSubmitRequestID() = %q, want idem-1", got)
	}
}

func TestResolveSubmitRequestIDFallsBackToRequestID(t *testing.T) {
	t.Parallel()

	got := ResolveSubmitRequestID("", " req-2 ")
	if got != "req-2" {
		t.Fatalf("ResolveSubmitRequestID() = %q, want req-2", got)
	}
}

func TestDeriveWorkflowRequestIDNormalizesFields(t *testing.T) {
	t.Parallel()

	requestedAt := time.Date(2026, 6, 15, 13, 14, 15, 123456789, time.FixedZone("CST", 8*3600))
	got := DeriveWorkflowRequestID(" task-1 ", " SAVE_DRAFT ", requestedAt)
	want := "temporal:task-1:save_draft:20260615T051415.123456789Z"
	if got != want {
		t.Fatalf("DeriveWorkflowRequestID() = %q, want %q", got, want)
	}
}

func TestDeriveWorkflowRequestIDUsesDefaults(t *testing.T) {
	t.Parallel()

	requestedAt := time.Date(2026, 6, 15, 5, 0, 0, 0, time.UTC)
	got := DeriveWorkflowRequestID("", "", requestedAt)
	want := "temporal:unknown-task:publish:20260615T050000.000000000Z"
	if got != want {
		t.Fatalf("DeriveWorkflowRequestID() = %q, want %q", got, want)
	}
}
