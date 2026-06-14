package submission

import (
	"errors"
	"testing"
	"time"
)

func TestBuildAttemptEventDraftUsesResolvedOutcome(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 14, 22, 30, 0, 0, time.UTC)
	draft := BuildAttemptEventDraft(
		&EventRecordState{
			Status:         "success",
			RequestID:      "req-1",
			Phase:          "persist_result",
			RemoteRecordID: "record-1",
		},
		nil,
		&ResponseOutcome{ValidationNotes: []string{"ok"}},
		nil,
		finishedAt,
	)

	if draft.Status != "success" || draft.RequestID != "req-1" || draft.Phase != "persist_result" {
		t.Fatalf("draft = %+v", draft)
	}
	if draft.RemoteRecordID != "record-1" {
		t.Fatalf("remoteRecordID = %q, want record-1", draft.RemoteRecordID)
	}
	if len(draft.ValidationNotes) != 1 || draft.ValidationNotes[0] != "ok" {
		t.Fatalf("validation notes = %+v, want [ok]", draft.ValidationNotes)
	}
	if !draft.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finishedAt = %v, want %v", draft.FinishedAt, finishedAt)
	}
}

func TestBuildAttemptEventDraftPrefersSubmitError(t *testing.T) {
	t.Parallel()

	draft := BuildAttemptEventDraft(nil, nil, nil, errors.New("boom"), time.Now())
	if draft.Status != "failed" || draft.ErrorMessage != "boom" {
		t.Fatalf("draft = %+v, want failed/boom", draft)
	}
}

func TestBuildPhaseEventDraftUsesFallbackDetailAndError(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 14, 22, 35, 0, 0, time.UTC)
	draft := BuildPhaseEventDraft("", "", "prepare payload", errors.New("bad"), finishedAt)

	if draft.Status != "running" || draft.Detail != "prepare payload" || draft.ErrorMessage != "bad" {
		t.Fatalf("draft = %+v, want running/prepare payload/bad", draft)
	}
	if !draft.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finishedAt = %v, want %v", draft.FinishedAt, finishedAt)
	}
}
