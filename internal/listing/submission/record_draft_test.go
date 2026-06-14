package submission

import (
	"errors"
	"testing"
	"time"
)

func TestBuildAttemptRecordDraftUsesResolvedAttemptState(t *testing.T) {
	t.Parallel()

	submittedAt := time.Date(2026, 6, 14, 22, 0, 0, 0, time.UTC)
	draft := BuildAttemptRecordDraft("publish", &ResponseOutcome{Success: true}, nil, submittedAt)

	if draft.Action != "publish" || draft.Status != "success" || draft.Error != "" {
		t.Fatalf("draft = %+v, want publish/success/empty-error", draft)
	}
	if !draft.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("submittedAt = %v, want %v", draft.SubmittedAt, submittedAt)
	}
}

func TestBuildAttemptRecordDraftKeepsFailureError(t *testing.T) {
	t.Parallel()

	draft := BuildAttemptRecordDraft("publish", &ResponseOutcome{Success: true}, errors.New("boom"), time.Now())
	if draft.Status != "failed" || draft.Error != "boom" {
		t.Fatalf("draft = %+v, want failed/boom", draft)
	}
}
