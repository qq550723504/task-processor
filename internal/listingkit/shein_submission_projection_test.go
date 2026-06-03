package listingkit

import (
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSheinSubmissionProjectionPrefersLatestOutcomeEvent(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	projection := buildSheinSubmissionProjection(&SheinPackage{
		SubmissionEvents: []sheinpub.SubmissionEvent{
			{Action: "save_draft", Status: "success"},
		},
		SubmissionState: &sheinpub.SubmissionReport{
			RemoteStatus: sheinpub.SubmissionRemoteStatusPending,
			Publish: &sheinpub.SubmissionRecord{
				Status:          sheinpub.SubmissionStatusSuccess,
				RemoteRecordID:  "record-1",
				RemoteCheckedAt: &checkedAt,
			},
		},
	})
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if projection.StatusFields.SheinWorkflowStatus != SheinWorkflowStatusDraftSaved {
		t.Fatalf("workflow status = %q", projection.StatusFields.SheinWorkflowStatus)
	}
	if projection.StatusFields.SheinLatestSubmissionStatus != "success" {
		t.Fatalf("latest status = %q", projection.StatusFields.SheinLatestSubmissionStatus)
	}
	if projection.StatusFields.SheinSubmissionRemoteStatus != sheinpub.SubmissionRemoteStatusPending {
		t.Fatalf("remote status = %q", projection.StatusFields.SheinSubmissionRemoteStatus)
	}
	if projection.TaskList.SheinSubmissionRemoteRecordID != "record-1" {
		t.Fatalf("remote record id = %q", projection.TaskList.SheinSubmissionRemoteRecordID)
	}
	if projection.TaskList.SheinSubmissionRemoteCheckedAt == nil || !projection.TaskList.SheinSubmissionRemoteCheckedAt.Equal(checkedAt) {
		t.Fatalf("remote checked at = %v", projection.TaskList.SheinSubmissionRemoteCheckedAt)
	}
}

func TestBuildSheinSubmissionProjectionNilPackage(t *testing.T) {
	t.Parallel()

	if projection := buildSheinSubmissionProjection(nil); projection != nil {
		t.Fatalf("projection = %+v, want nil", projection)
	}
}
