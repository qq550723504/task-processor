package submission

import (
	"errors"
	"testing"
)

func TestResolveAttemptResultStatePrefersSubmitError(t *testing.T) {
	t.Parallel()

	got := ResolveAttemptResultState("publish", &ResponseOutcome{Success: true}, errors.New("boom"))
	if got.Status != "failed" || got.ErrorMessage != "boom" {
		t.Fatalf("state = %+v, want failed/boom", got)
	}
}

func TestResolveAttemptResultStateMarksPublishSuccess(t *testing.T) {
	t.Parallel()

	got := ResolveAttemptResultState("publish", &ResponseOutcome{Success: true}, nil)
	if got.Status != "success" || got.ErrorMessage != "" {
		t.Fatalf("state = %+v, want success with empty error", got)
	}
}

func TestResolveAttemptResultStateMarksSaveDraftCodeZeroSuccess(t *testing.T) {
	t.Parallel()

	got := ResolveAttemptResultState("save_draft", &ResponseOutcome{Code: "0"}, nil)
	if got.Status != "success" || got.ErrorMessage != "" {
		t.Fatalf("state = %+v, want success with empty error", got)
	}
}

func TestResolveAttemptResultStateFallsBackToUnknown(t *testing.T) {
	t.Parallel()

	got := ResolveAttemptResultState("publish", &ResponseOutcome{Code: "E1", Message: "bad"}, nil)
	if got.Status != "unknown" || got.ErrorMessage != "" {
		t.Fatalf("state = %+v, want unknown with empty error", got)
	}
}
