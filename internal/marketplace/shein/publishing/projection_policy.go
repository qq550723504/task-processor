package publishing

import listingsubmission "task-processor/internal/listing/submission"

func SubmissionProjectionWorkflowPolicy(successStatus, failedStatus string) listingsubmission.ProjectionWorkflowPolicy {
	return listingsubmission.ProjectionWorkflowPolicy{
		SuccessStatus:            successStatus,
		FailedStatus:             failedStatus,
		PublishedWorkflowStatus:  "published",
		DraftSavedWorkflowStatus: "draft_saved",
		FailedWorkflowStatus:     "publish_failed",
		ReadyWorkflowStatus:      "ready_to_submit",
		PendingWorkflowStatus:    "pending_confirmation",
	}
}
