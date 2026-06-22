package listingkit

import (
	"context"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

type sheinRemoteCompletionState struct {
	taskID    string
	task      *Task
	pkg       *SheinPackage
	action    string
	requestID string
	startedAt time.Time
	response  *sheinpub.SubmissionResponse
}

func persistSheinRemoteCompletionFailure(
	ctx context.Context,
	saveTaskResult func(context.Context, string, *ListingKitResult) error,
	state *sheinRemoteCompletionState,
	phase string,
	remoteErr error,
) error {
	if state == nil || state.task == nil || state.pkg == nil {
		return remoteErr
	}
	_, event := sheinpub.FailSubmitAttemptWithResponseAndBuildEvent(state.pkg, state.taskID, state.action, state.requestID, phase, state.response, remoteErr, time.Now())
	sheinpub.AppendSubmissionEvent(state.pkg, event)
	if state.task.Result == nil {
		return remoteErr
	}
	state.task.Result.UpdatedAt = time.Now()
	if saveTaskResult == nil {
		return nil
	}
	if err := saveTaskResult(ctx, state.taskID, state.task.Result); err != nil {
		return err
	}
	return remoteErr
}

func persistSheinRemoteCompletionSuccess(
	ctx context.Context,
	state *sheinRemoteCompletionState,
	response *sheinpub.SubmissionResponse,
	rememberSubmitted func(*Task, string),
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error,
) (*sheinpub.SubmissionRecord, error) {
	if state == nil || state.task == nil || state.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	record, event := sheinpub.CompleteSubmitAttemptAndBuildEvent(state.pkg, state.taskID, state.action, state.requestID, response, nil, state.startedAt, time.Now())
	sheinpub.AppendSubmissionEvent(state.pkg, event)
	if rememberSubmitted != nil {
		rememberSubmitted(state.task, state.action)
	}
	if persistSuccessfulSubmission != nil {
		if err := persistSuccessfulSubmission(ctx, state.taskID, state.task, state.action); err != nil {
			return record, err
		}
	}
	return record, nil
}
