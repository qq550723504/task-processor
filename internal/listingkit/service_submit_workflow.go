package listingkit

import (
	"context"
	"strings"
)

func (s *service) submitSheinTaskWithWorkflow(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
	err := s.sheinPublishWorkflowClient.StartSheinPublish(ctx, SheinPublishWorkflowStartInput{
		TaskID:         strings.TrimSpace(taskID),
		Platform:       opts.platform,
		Action:         opts.action,
		RequestID:      opts.requestID,
		ConfirmedFinal: req != nil && req.ConfirmedFinal,
		RequestedAt:    opts.startedAt,
	})
	if err == nil {
		return s.GetTaskPreview(ctx, taskID, "shein")
	}
	if shouldReplayStartedTemporalSubmit(err, opts.requestID) {
		return s.buildTaskPreview(ctx, task, "shein")
	}
	return nil, s.taskSubmissionRecoveryOrDefault().handleSheinWorkflowStartFailure(ctx, taskID, task, opts, err)
}
