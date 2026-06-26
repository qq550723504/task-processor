package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinRemoteCompletionState = submissiondomain.RemoteCompletionState[*Task, *SheinPackage, *sheinpub.SubmissionResponse]

func persistSheinRemoteCompletionFailure(
	ctx context.Context,
	saveTaskResult func(context.Context, string, *ListingKitResult) error,
	state *sheinRemoteCompletionState,
	phase string,
	remoteErr error,
) error {
	if state == nil || state.Task == nil || state.Package == nil {
		return remoteErr
	}
	_, event := sheinpub.FailSubmitAttemptWithResponseAndBuildEvent(state.Package, state.TaskID, state.Action, state.RequestID, phase, state.Response, remoteErr, time.Now())
	sheinpub.AppendSubmissionEvent(state.Package, event)
	if state.Task.Result == nil {
		return remoteErr
	}
	state.Task.Result.UpdatedAt = time.Now()
	if saveTaskResult == nil {
		return nil
	}
	if err := saveTaskResult(ctx, state.TaskID, state.Task.Result); err != nil {
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
	if state == nil || state.Task == nil || state.Package == nil {
		return nil, ErrTaskResultUnavailable
	}
	record, event := sheinpub.CompleteSubmitAttemptAndBuildEvent(state.Package, state.TaskID, state.Action, state.RequestID, response, nil, state.StartedAt, time.Now())
	sheinpub.AppendSubmissionEvent(state.Package, event)
	if rememberSubmitted != nil {
		rememberSubmitted(state.Task, state.Action)
	}
	if persistSuccessfulSubmission != nil {
		if err := persistSuccessfulSubmission(ctx, state.TaskID, state.Task, state.Action); err != nil {
			return record, err
		}
	}
	return record, nil
}
