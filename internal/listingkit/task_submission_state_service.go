package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

type taskSubmissionStateServiceConfig struct {
	repo                   Repository
	rememberSheinSubmitted func(*Task, string)
}

type taskSubmissionStateService struct {
	repo                   Repository
	rememberSheinSubmitted func(*Task, string)
	resultRunner           *submissiondomain.ResultPersistenceService[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner          *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

func newTaskSubmissionStateService(config taskSubmissionStateServiceConfig) *taskSubmissionStateService {
	service := &taskSubmissionStateService{
		repo:                   config.repo,
		rememberSheinSubmitted: config.rememberSheinSubmitted,
	}
	successRunner := newSheinSubmissionSuccessPersistenceService(
		service.completeDirectSubmitAttempt,
		nil,
		service.rememberSheinSubmitted,
		service.persistSuccessfulSheinSubmission,
	)
	failureRunner := newSheinSubmissionFailurePersistenceService(service.recordFailureState)
	service.failureRunner = failureRunner
	service.resultRunner = submissiondomain.NewResultPersistenceService(submissiondomain.ResultPersistenceServiceConfig[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		SuccessRunner: successRunner,
		FailureRunner: failureRunner,
		BuildSuccessInput: func(in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse] {
			return submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
				TaskID:    in.TaskID,
				Task:      in.Task,
				Package:   in.Package,
				Action:    in.Action,
				RequestID: in.RequestID,
				Response:  in.Response,
				StartedAt: in.StartedAt,
			}
		},
		BuildFailureInput: func(in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage] {
			return submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
				TaskID:    in.TaskID,
				Result:    in.Result,
				Package:   in.Package,
				Action:    in.Action,
				RequestID: in.RequestID,
				Phase:     in.Phase,
				Err:       in.Err,
			}
		},
		BeforeFailure: service.prepareDirectFailurePersistence,
		FallbackSuccess: func(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
			return service.persistDirectSuccessFallback(ctx, in)
		},
		FallbackFailure: func(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
			return service.persistDirectFailureFallback(ctx, in)
		},
		ReturnOriginalFailure: true,
	})
	return service
}

func (s *taskSubmissionStateService) persistSuccessfulSheinSubmission(ctx context.Context, taskID string, task *Task, action string) error {
	if task == nil || task.Result == nil {
		return nil
	}
	if success := sheinpub.SubmissionSucceeded(task.Result.Shein, action); success {
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
	sheinpub.AppendSubmissionEvent(pkg, advanceSheinSubmitPhaseAndBuildEvent(pkg, taskID, action, requestID, phase, time.Now()))
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
	sheinpub.SetSubmissionRemoteResponse(pkg, opts.action, opts.requestID, supplierCode, response)
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return err
	}
	return s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePersistResult)
}
