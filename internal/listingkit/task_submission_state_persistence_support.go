package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskSubmissionStateService) finishSheinDirectSubmitAttempt(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {
	var result *ListingKitResult
	if task != nil {
		result = task.Result
	}
	return s.resultRunner.Finish(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    taskID,
		Task:      task,
		Result:    result,
		Package:   pkg,
		Action:    opts.action,
		RequestID: opts.requestID,
		Phase:     sheinpub.SubmissionPhasePersistResult,
		Response:  response,
		StartedAt: opts.startedAt,
		Err:       responseErr,
	})
}

func (s *taskSubmissionStateService) completeDirectSubmitAttempt(in submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse], finishedAt time.Time) {
	_, event := sheinpub.CompleteSubmitAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Response, nil, in.StartedAt, finishedAt)
	sheinpub.AppendSubmissionEvent(in.Package, event)
}

func (s *taskSubmissionStateService) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {
	if s.failureRunner != nil {
		return s.failureRunner.PersistFailure(ctx, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
			TaskID:    taskID,
			Result:    result,
			Package:   pkg,
			Action:    action,
			RequestID: requestedID,
			Phase:     phase,
			Err:       submitErr,
		})
	}
	return s.recordFailureState(ctx, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
		TaskID:    taskID,
		Result:    result,
		Package:   pkg,
		Action:    action,
		RequestID: requestedID,
		Phase:     phase,
		Err:       submitErr,
	})
}

func (s *taskSubmissionStateService) prepareDirectFailurePersistence(in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) {
	if in.Package == nil {
		return
	}
	sheinpub.SetSubmissionRemoteResponse(in.Package, in.Action, in.RequestID, "", in.Response)
}

func (s *taskSubmissionStateService) persistDirectSuccessFallback(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
	success := submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    in.TaskID,
		Task:      in.Task,
		Package:   in.Package,
		Action:    in.Action,
		RequestID: in.RequestID,
		Response:  in.Response,
		StartedAt: in.StartedAt,
	}
	s.completeDirectSubmitAttempt(success, time.Now())
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(in.Task, in.Action)
	}
	return s.persistSuccessfulSheinSubmission(ctx, in.TaskID, in.Task, in.Action)
}

func (s *taskSubmissionStateService) persistDirectFailureFallback(_ context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
	_, event := sheinpub.CompleteSubmitAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Response, in.Err, in.StartedAt, time.Now())
	sheinpub.AppendSubmissionEvent(in.Package, event)
	return nil
}

func (s *taskSubmissionStateService) failSheinDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {
	if task == nil {
		return submitErr
	}
	if saveErr := s.recordSheinSubmissionFailureForState(ctx, taskID, task.Result, pkg, action, "", "", submitErr); saveErr != nil {
		return saveErr
	}
	return submitErr
}

func (s *taskSubmissionStateService) recordFailureState(ctx context.Context, in submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error {
	_, event := sheinpub.FailSubmitAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Phase, in.Err, time.Now())
	sheinpub.AppendSubmissionEvent(in.Package, event)
	if in.Result == nil {
		return nil
	}
	in.Result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, in.TaskID, in.Result)
}
