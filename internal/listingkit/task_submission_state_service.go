package listingkit

import (
	"context"
	"time"

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
}

func newTaskSubmissionStateService(config taskSubmissionStateServiceConfig) *taskSubmissionStateService {
	return &taskSubmissionStateService{
		repo:                   config.repo,
		rememberSheinSubmitted: config.rememberSheinSubmitted,
	}
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
	_, event := listingsubmission.CompleteAttemptAndBuildEvent(pkg, taskID, opts.action, opts.requestID, response, responseErr, opts.startedAt, time.Now())
	appendSheinSubmissionEvent(pkg, event)
	if responseErr == nil && s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, opts.action)
	}
	if err := s.persistSuccessfulSheinSubmission(ctx, taskID, task, opts.action); err != nil {
		return err
	}
	return responseErr
}

func (s *taskSubmissionStateService) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {
	return s.recordSheinSubmissionFailureForState(ctx, taskID, result, pkg, action, "", "", submitErr)
}

func (s *taskSubmissionStateService) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {
	_, event := listingsubmission.FailAttemptAndBuildEvent(pkg, taskID, action, requestedID, phase, submitErr, time.Now())
	appendSheinSubmissionEvent(pkg, event)
	if result == nil {
		return nil
	}
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
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
