package listingkit

import (
	"context"
)

func (s *service) submitSheinTaskWithWorkflow(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
	return s.taskTemporalSubmissionAdapterOrDefault().startSheinPublishWorkflowAttempt(ctx, taskID, task, req, opts)
}
