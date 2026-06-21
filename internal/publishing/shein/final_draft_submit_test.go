package shein

import (
	"testing"
	"time"
)

func TestConfirmFinalSubmissionDraftInitializesAndMarksDraft(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 14, 0, 0, 0, time.UTC)
	pkg := &Package{}

	draft := ConfirmFinalSubmissionDraft(pkg, "publish", now)

	if draft == nil {
		t.Fatal("draft = nil, want initialized final draft")
	}
	if pkg.FinalSubmissionDraft != draft {
		t.Fatalf("FinalSubmissionDraft = %p, want returned draft %p", pkg.FinalSubmissionDraft, draft)
	}
	if !draft.Confirmed {
		t.Fatal("Confirmed = false, want true")
	}
	if draft.ConfirmedAt == nil || !draft.ConfirmedAt.Equal(now) {
		t.Fatalf("ConfirmedAt = %v, want %v", draft.ConfirmedAt, now)
	}
	if draft.UpdatedAt == nil || !draft.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", draft.UpdatedAt, now)
	}
	if draft.SubmitMode != "publish" {
		t.Fatalf("SubmitMode = %q, want publish", draft.SubmitMode)
	}
}

func TestConfirmFinalSubmissionDraftPreservesExistingSubmitMode(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 15, 0, 0, 0, time.UTC)
	pkg := &Package{FinalSubmissionDraft: &FinalDraft{SubmitMode: "save_draft"}}

	draft := ConfirmFinalSubmissionDraft(pkg, "publish", now)

	if draft.SubmitMode != "save_draft" {
		t.Fatalf("SubmitMode = %q, want existing save_draft", draft.SubmitMode)
	}
	if draft.ConfirmedAt == nil || !draft.ConfirmedAt.Equal(now) {
		t.Fatalf("ConfirmedAt = %v, want %v", draft.ConfirmedAt, now)
	}
}
