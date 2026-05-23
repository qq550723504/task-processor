package listingkit

import (
	"context"
	"errors"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
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
	return nil, s.handleSheinWorkflowStartFailure(ctx, taskID, task, opts, err)
}

func (s *service) handleSheinWorkflowStartFailure(ctx context.Context, taskID string, task *Task, opts sheinWorkflowSubmitOptions, startErr error) error {
	var result *ListingKitResult
	var pkg *SheinPackage
	if task != nil {
		result = task.Result
		if task.Result != nil {
			pkg = task.Result.Shein
		}
	}
	failErr := s.recordSheinSubmissionFailureForState(
		ctx,
		taskID,
		result,
		pkg,
		opts.action,
		opts.requestID,
		sheinpub.SubmissionPhaseValidate,
		startErr,
	)
	clearErr := s.clearSheinSubmitLeaseAfterStartFailure(ctx, taskID, opts.action, opts.requestID, startErr)
	if failErr != nil {
		if clearErr != nil {
			return errors.Join(failErr, clearErr)
		}
		return failErr
	}
	if clearErr != nil {
		return clearErr
	}
	return startErr
}
