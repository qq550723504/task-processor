package submission

import (
	"errors"
	"testing"
)

func TestResolveSubmitTargetUsesFallbacks(t *testing.T) {
	t.Parallel()

	target := ResolveSubmitTarget("", "", "shein", "")
	if target.Platform != "shein" || target.Action != "publish" {
		t.Fatalf("ResolveSubmitTarget() = %+v, want shein/publish", target)
	}
}

func TestResolveSubmitTargetNormalizesRequestedValues(t *testing.T) {
	t.Parallel()

	target := ResolveSubmitTarget(" SHEIN ", " SAVE_DRAFT ", "ignored", "publish")
	if target.Platform != "shein" || target.Action != "save_draft" {
		t.Fatalf("ResolveSubmitTarget() = %+v, want trimmed lowercase request values", target)
	}
}

func TestResolveSubmitTargetUsesDefaultActionWhenRequestMissing(t *testing.T) {
	t.Parallel()

	target := ResolveSubmitTarget("", "", "shein", " save_draft ")
	if target.Action != "save_draft" {
		t.Fatalf("action = %q, want save_draft", target.Action)
	}
}

func TestIsReplayOfStartedSubmitMatchesRequestID(t *testing.T) {
	t.Parallel()

	err := &SubmitInProgressError{RequestID: "req-1"}
	if !IsReplayOfStartedSubmit(err, " req-1 ") {
		t.Fatal("IsReplayOfStartedSubmit() = false, want true")
	}
}

func TestIsReplayOfStartedSubmitRejectsDifferentRequestID(t *testing.T) {
	t.Parallel()

	err := &SubmitInProgressError{RequestID: "req-1"}
	if IsReplayOfStartedSubmit(err, "req-2") {
		t.Fatal("IsReplayOfStartedSubmit() = true, want false")
	}
}

func TestIsReplayOfStartedSubmitRejectsNonSubmitInProgressErrors(t *testing.T) {
	t.Parallel()

	if IsReplayOfStartedSubmit(errors.New("boom"), "req-1") {
		t.Fatal("IsReplayOfStartedSubmit() = true, want false")
	}
}
