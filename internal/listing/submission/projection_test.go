package submission

import (
	"testing"
	"time"
)

var testProjectionPolicy = ProjectionWorkflowPolicy{
	SuccessStatus:            "success",
	FailedStatus:             "failed",
	PublishedWorkflowStatus:  "published",
	DraftSavedWorkflowStatus: "draft_saved",
	FailedWorkflowStatus:     "publish_failed",
	ReadyWorkflowStatus:      "ready_to_submit",
	PendingWorkflowStatus:    "pending_confirmation",
}

func TestLatestOutcomeEventSkipsPhaseEvents(t *testing.T) {
	t.Parallel()

	got := LatestOutcomeEvent([]Event{
		{Action: SubmitActionPhase, Status: "running"},
		{Action: SubmitActionPublish, Status: "failed", ErrorMessage: "remote failed"},
		{Action: SubmitActionSaveDraft, Status: "success"},
	})

	if got == nil || got.Action != SubmitActionPublish || got.ErrorMessage != "remote failed" {
		t.Fatalf("LatestOutcomeEvent() = %+v, want first non-phase event", got)
	}
}

func TestResolveProjectionPrefersLatestOutcomeEvent(t *testing.T) {
	t.Parallel()

	projection := ResolveProjection(
		[]Event{{Action: SubmitActionPublish, Status: "failed", ErrorMessage: "bad image"}},
		&Report{
			LastAction: SubmitActionSaveDraft,
			LastStatus: "success",
			SaveDraft:  &ActionRecord{Status: "success", RemoteRecordID: "draft-1"},
		},
		true,
		testProjectionPolicy,
	)

	if projection.WorkflowStatus != "publish_failed" {
		t.Fatalf("workflow status = %q, want publish_failed", projection.WorkflowStatus)
	}
	if projection.LatestStatus != "failed" || projection.LatestError != "bad image" {
		t.Fatalf("latest = %q/%q, want failed/bad image", projection.LatestStatus, projection.LatestError)
	}
}

func TestResolveProjectionFallsBackToReportRecord(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 17, 10, 30, 0, 0, time.UTC)
	projection := ResolveProjection(
		nil,
		&Report{
			LastAction:   SubmitActionPublish,
			LastStatus:   "success",
			RemoteStatus: "confirmed",
			Publish: &ActionRecord{
				Status:          "success",
				RemoteRecordID:  "remote-1",
				RemoteCheckedAt: &checkedAt,
			},
		},
		false,
		testProjectionPolicy,
	)

	if projection.WorkflowStatus != "published" {
		t.Fatalf("workflow status = %q, want published", projection.WorkflowStatus)
	}
	if projection.RemoteStatus != "confirmed" || projection.RemoteRecordID != "remote-1" {
		t.Fatalf("remote = %q/%q, want confirmed/remote-1", projection.RemoteStatus, projection.RemoteRecordID)
	}
	if projection.RemoteCheckedAt == nil || !projection.RemoteCheckedAt.Equal(checkedAt) {
		t.Fatalf("remote checked at = %+v, want record checked time", projection.RemoteCheckedAt)
	}
}

func TestResolveProjectionUsesReadinessWhenNoSubmissionState(t *testing.T) {
	t.Parallel()

	readyProjection := ResolveProjection(nil, nil, true, testProjectionPolicy)
	pendingProjection := ResolveProjection(nil, nil, false, testProjectionPolicy)

	if readyProjection.WorkflowStatus != "ready_to_submit" {
		t.Fatalf("ready workflow status = %q, want ready_to_submit", readyProjection.WorkflowStatus)
	}
	if pendingProjection.WorkflowStatus != "pending_confirmation" {
		t.Fatalf("pending workflow status = %q, want pending_confirmation", pendingProjection.WorkflowStatus)
	}
}
