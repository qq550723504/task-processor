package submission

import (
	"errors"
	"testing"
	"time"

	"task-processor/internal/listingkit/core"
)

func TestSubmitInProgressErrorWithLeaseExpiry(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 14, 15, 4, 5, 0, time.UTC)
	err := (&SubmitInProgressError{
		Platform:       "shein",
		Action:         "publish",
		Phase:          "confirm_remote",
		LeaseExpiresAt: &expiresAt,
	}).Error()

	want := "submit already in progress: shein publish is in confirm_remote until 2026-06-14T15:04:05Z"
	if err != want {
		t.Fatalf("Error() = %q, want %q", err, want)
	}
}

func TestSubmitInProgressErrorWithoutLeaseExpiry(t *testing.T) {
	t.Parallel()

	err := (&SubmitInProgressError{
		Platform: "shein",
		Action:   "save_draft",
		Phase:    "validate",
	}).Error()

	want := "submit already in progress: shein save_draft is in validate"
	if err != want {
		t.Fatalf("Error() = %q, want %q", err, want)
	}
}

func TestSubmitInProgressErrorUnwrapsCoreError(t *testing.T) {
	t.Parallel()

	err := &SubmitInProgressError{}
	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("errors.Is(%v, core.ErrSubmitInProgress) = false, want true", err)
	}
}

func TestNewSubmitInProgressErrorTrimsFields(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 14, 15, 4, 5, 0, time.UTC)
	err := NewSubmitInProgressError(" shein ", " publish ", " confirm_remote ", " req-1 ", &expiresAt)

	if err.Platform != "shein" || err.Action != "publish" || err.Phase != "confirm_remote" || err.RequestID != "req-1" {
		t.Fatalf("NewSubmitInProgressError() = %+v, want trimmed fields", err)
	}
	if err.LeaseExpiresAt != &expiresAt {
		t.Fatalf("LeaseExpiresAt = %v, want %v", err.LeaseExpiresAt, &expiresAt)
	}
}
