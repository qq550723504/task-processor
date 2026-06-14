package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	listingsubmission "task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

type taskSubmissionStateServiceConfig struct {
	repo                   Repository
	rememberSheinSubmitted func(*Task, string)
}

type taskSubmissionStateService struct {
	repo                   Repository
	rememberSheinSubmitted func(*Task, string)
	successRunner          *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner          *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

func newTaskSubmissionStateService(config taskSubmissionStateServiceConfig) *taskSubmissionStateService {
	service := &taskSubmissionStateService{
		repo:                   config.repo,
		rememberSheinSubmitted: config.rememberSheinSubmitted,
	}
	service.successRunner = newSheinSubmissionSuccessPersistenceService(
		service.completeDirectSubmitAttempt,
		nil,
		service.rememberSheinSubmitted,
		service.persistSuccessfulSheinSubmission,
	)
	service.failureRunner = newSheinSubmissionFailurePersistenceService(service.recordFailureState)
	return service
}

func (s *taskSubmissionStateService) persistSuccessfulSheinSubmission(ctx context.Context, taskID string, task *Task, action string) error {
	if task == nil || task.Result == nil {
		return nil
	}
	if success := listingsubmission.SubmissionSucceeded(task.Result.Shein, action); success {
		markTaskCompleted(task)
		return s.repo.MarkCompleted(ctx, taskID, task.Result)
	}
	task.Result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, task.Result)
}

func (s *taskSubmissionStateService) persistSheinDirectSubmitPhase(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, phase string) error {
	var result *ListingKitResult
	if task != nil {
		result = task.Result
	}
	return s.persistSheinSubmitPhase(ctx, taskID, result, pkg, opts.action, opts.requestID, phase)
}

func (s *taskSubmissionStateService) persistSheinSubmitPhase(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string) error {
	appendSheinSubmissionEvent(pkg, listingsubmission.AdvancePhaseAndBuildEvent(pkg, taskID, action, requestID, phase, time.Now(), sheinSubmitInFlightTTL))
	if result == nil {
		return nil
	}
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
}

func (s *taskSubmissionStateService) persistSuccessfulSheinDirectResponse(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, supplierCode string, response *sheinpub.SubmissionResponse) error {
	if task == nil || task.Result == nil {
		return nil
	}
	setSheinSubmitRemoteResponse(pkg, opts.action, opts.requestID, supplierCode, response)
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return err
	}
	return s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePersistResult)
}

func (s *taskSubmissionStateService) finishSheinDirectSubmitAttempt(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {
	if responseErr == nil && s.successRunner != nil {
		if err := s.successRunner.PersistSuccess(ctx, submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
			TaskID:    taskID,
			Task:      task,
			Package:   pkg,
			Action:    opts.action,
			RequestID: opts.requestID,
			Response:  response,
			StartedAt: opts.startedAt,
		}); err != nil {
			return err
		}
		return nil
	}
	if s.failureRunner != nil {
		setSheinSubmitRemoteResponse(pkg, opts.action, opts.requestID, "", response)
		var result *ListingKitResult
		if task != nil {
			result = task.Result
		}
		if err := s.failureRunner.PersistFailure(ctx, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
			TaskID:    taskID,
			Result:    result,
			Package:   pkg,
			Action:    opts.action,
			RequestID: opts.requestID,
			Phase:     sheinpub.SubmissionPhasePersistResult,
			Err:       responseErr,
		}); err != nil {
			return err
		}
		return responseErr
	}
	_, event := listingsubmission.CompleteAttemptAndBuildEvent(pkg, taskID, opts.action, opts.requestID, response, responseErr, opts.startedAt, time.Now())
	appendSheinSubmissionEvent(pkg, event)
	return responseErr
}

func (s *taskSubmissionStateService) completeDirectSubmitAttempt(in submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse], finishedAt time.Time) {
	_, event := listingsubmission.CompleteAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Response, nil, in.StartedAt, finishedAt)
	appendSheinSubmissionEvent(in.Package, event)
}

func (s *taskSubmissionStateService) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {
	return s.recordSheinSubmissionFailureForState(ctx, taskID, result, pkg, action, "", "", submitErr)
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

func (s *taskSubmissionStateService) failSheinDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {
	if task == nil {
		return submitErr
	}
	if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, submitErr); saveErr != nil {
		return saveErr
	}
	return submitErr
}

func (s *taskSubmissionStateService) recordFailureState(ctx context.Context, in submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error {
	_, event := listingsubmission.FailAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Phase, in.Err, time.Now())
	appendSheinSubmissionEvent(in.Package, event)
	if in.Result == nil {
		return nil
	}
	in.Result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, in.TaskID, in.Result)
}
