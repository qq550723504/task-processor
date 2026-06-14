package submission

import (
	"strings"
	"testing"
)

func TestSaveDraftSucceeded(t *testing.T) {
	t.Parallel()

	if !SaveDraftSucceeded("save_draft", &ResponseOutcome{Code: "0"}) {
		t.Fatal("expected save draft response code 0 to succeed")
	}
	if SaveDraftSucceeded("publish", &ResponseOutcome{Code: "0"}) {
		t.Fatal("publish action should not use save draft success policy")
	}
	if SaveDraftSucceeded("save_draft", nil) {
		t.Fatal("nil outcome should not succeed")
	}
}

func TestBuildResponseError(t *testing.T) {
	t.Parallel()

	if err := BuildResponseError("SHEIN", "publish", &ResponseOutcome{Success: true}); err != nil {
		t.Fatalf("BuildResponseError(success) = %v, want nil", err)
	}
	if err := BuildResponseError("SHEIN", "save_draft", &ResponseOutcome{Code: "0"}); err != nil {
		t.Fatalf("BuildResponseError(save draft code 0) = %v, want nil", err)
	}

	err := BuildResponseError("SHEIN", "publish", &ResponseOutcome{
		ValidationNotes: []string{"missing title", "bad image"},
	})
	if err == nil || !strings.Contains(err.Error(), "SHEIN publish pre-validation failed: missing title; bad image") {
		t.Fatalf("BuildResponseError(validation notes) = %v", err)
	}

	err = BuildResponseError("SHEIN", "publish", &ResponseOutcome{Message: "remote rejected"})
	if err == nil || !strings.Contains(err.Error(), "SHEIN publish did not complete: remote rejected") {
		t.Fatalf("BuildResponseError(message) = %v", err)
	}
}
