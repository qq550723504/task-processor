package listingkit

import (
	"context"
)

func (s *service) submitSheinTaskWithWorkflow(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
	temporal := s.taskTemporalSubmissionOrDefault()
	if temporal == nil {
		return nil, ErrTaskResultUnavailable
	}
	return temporal.StartSheinPublishWorkflowAttempt(ctx, taskID, task, req, opts)
}

func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {
	client, enabled := resolveSubmissionWorkflowClient(s)
	return s != nil &&
		enabled &&
		client != nil &&
		platform == "shein" &&
		action == "publish"
}
