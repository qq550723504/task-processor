package publishing

import "testing"

func TestSubmissionProjectionWorkflowPolicyUsesSheinWorkflowStatuses(t *testing.T) {
	t.Parallel()

	policy := SubmissionProjectionWorkflowPolicy("success", "failed")
	if policy.SuccessStatus != "success" {
		t.Fatalf("SuccessStatus = %q, want success", policy.SuccessStatus)
	}
	if policy.FailedStatus != "failed" {
		t.Fatalf("FailedStatus = %q, want failed", policy.FailedStatus)
	}
	if policy.PublishedWorkflowStatus != "published" {
		t.Fatalf("PublishedWorkflowStatus = %q, want published", policy.PublishedWorkflowStatus)
	}
	if policy.DraftSavedWorkflowStatus != "draft_saved" {
		t.Fatalf("DraftSavedWorkflowStatus = %q, want draft_saved", policy.DraftSavedWorkflowStatus)
	}
	if policy.FailedWorkflowStatus != "publish_failed" {
		t.Fatalf("FailedWorkflowStatus = %q, want publish_failed", policy.FailedWorkflowStatus)
	}
	if policy.ReadyWorkflowStatus != "ready_to_submit" {
		t.Fatalf("ReadyWorkflowStatus = %q, want ready_to_submit", policy.ReadyWorkflowStatus)
	}
	if policy.PendingWorkflowStatus != "pending_confirmation" {
		t.Fatalf("PendingWorkflowStatus = %q, want pending_confirmation", policy.PendingWorkflowStatus)
	}
}
