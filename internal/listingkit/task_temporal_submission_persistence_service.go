package listingkit

import (
	"context"
	"errors"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

type taskTemporalSubmissionPersistenceServiceConfig struct {
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	rememberSheinSubmitted               func(*Task, string)
	successRunner                        *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner                        *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

type taskTemporalSubmissionPersistenceService struct {
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	rememberSheinSubmitted               func(*Task, string)
	successRunner                        *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner                        *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

func newTaskTemporalSubmissionPersistenceService(config taskTemporalSubmissionPersistenceServiceConfig) *taskTemporalSubmissionPersistenceService {
	service := &taskTemporalSubmissionPersistenceService{
		loadSheinPublishTask:                 config.loadSheinPublishTask,
		saveTaskResult:                       config.saveTaskResult,
		persistSheinSubmitPhase:              config.persistSheinSubmitPhase,
		persistSuccessfulSheinSubmission:     config.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: config.recordSheinSubmissionFailureForState,
		rememberSheinSubmitted:               config.rememberSheinSubmitted,
		successRunner:                        config.successRunner,
		failureRunner:                        config.failureRunner,
	}
	if service.successRunner == nil {
		service.successRunner = newSheinSubmissionSuccessPersistenceService(
			service.completeTemporalSubmitAttempt,
			service.persistTemporalSuccessResultAndPhase,
			service.rememberSheinSubmitted,
			service.persistSuccessfulSheinSubmission,
		)
	}
	if service.failureRunner == nil {
		service.failureRunner = newSheinSubmissionFailurePersistenceService(service.recordTemporalFailureState)
	}
	return service
}

func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	sheinpub.ApplySubmissionPersistenceInput(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot)
	return s.persistSheinTemporalSubmissionSuccess(ctx, in.TaskID, task, pkg, in.Action, in.RequestID, in.Response)
}

func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	sheinpub.ApplySubmissionPersistenceInput(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot)
	return s.persistSheinTemporalSubmissionFailure(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, in.Phase, in.ErrorMessage)
}

func (s *taskTemporalSubmissionPersistenceService) loadSheinPublishTaskState(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {
	if s == nil || s.loadSheinPublishTask == nil {
		return nil, nil, errors.New("shein publish task loader is not configured")
	}
	return s.loadSheinPublishTask(ctx, taskID)
}

func (s *taskTemporalSubmissionPersistenceService) persistSheinSubmitSnapshot(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID string, snapshot *sheinpub.SubmitSnapshot) error {
	if result == nil || pkg == nil || snapshot == nil {
		return nil
	}
	sheinpub.SetSubmissionSnapshot(pkg, action, requestID, snapshot)
	result.UpdatedAt = time.Now()
	if s.saveTaskResult == nil {
		return nil
	}
	return s.saveTaskResult(ctx, taskID, result)
}

func (s *taskTemporalSubmissionPersistenceService) persistSheinTemporalSubmissionSuccess(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse) error {
	startedAt := sheinpub.SubmissionStartedAt(pkg, action, requestID, time.Now())
	if s.successRunner != nil {
		return s.successRunner.PersistSuccess(ctx, submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
			TaskID:    taskID,
			Task:      task,
			Package:   pkg,
			Action:    action,
			RequestID: requestID,
			Response:  response,
			StartedAt: startedAt,
		})
	}
	task.Result.UpdatedAt = time.Now()
	if s.saveTaskResult != nil {
		if err := s.saveTaskResult(ctx, taskID, task.Result); err != nil {
			return err
		}
	}
	if s.persistSheinSubmitPhase != nil {
		if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePersistResult); err != nil {
			return err
		}
	}
	_, event := completeSheinSubmitAttemptAndBuildEvent(pkg, taskID, action, requestID, response, nil, startedAt, time.Now())
	appendSheinSubmissionEvent(pkg, event)
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if s.persistSuccessfulSheinSubmission != nil {
		return s.persistSuccessfulSheinSubmission(ctx, taskID, task, action)
	}
	return nil
}

func (s *taskTemporalSubmissionPersistenceService) persistTemporalSuccessResultAndPhase(ctx context.Context, in submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]) error {
	in.Task.Result.UpdatedAt = time.Now()
	if s.saveTaskResult != nil {
		if err := s.saveTaskResult(ctx, in.TaskID, in.Task.Result); err != nil {
			return err
		}
	}
	if s.persistSheinSubmitPhase == nil {
		return nil
	}
	return s.persistSheinSubmitPhase(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, sheinpub.SubmissionPhasePersistResult)
}

func (s *taskTemporalSubmissionPersistenceService) completeTemporalSubmitAttempt(in submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse], finishedAt time.Time) {
	_, event := completeSheinSubmitAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Response, nil, in.StartedAt, finishedAt)
	appendSheinSubmissionEvent(in.Package, event)
}

func (s *taskTemporalSubmissionPersistenceService) persistSheinTemporalSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase, errorMessage string) error {
	input := submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
		TaskID:    taskID,
		Result:    result,
		Package:   pkg,
		Action:    action,
		RequestID: requestID,
		Phase:     phase,
		Err:       errors.New(strings.TrimSpace(errorMessage)),
	}
	if s.failureRunner != nil {
		return s.failureRunner.PersistFailure(ctx, input)
	}
	return s.recordTemporalFailureState(ctx, input)
}

func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshFailure(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, remoteErr error) error {
	_, event := failSheinSubmitAttemptWithResponseAndBuildEvent(pkg, taskID, action, requestID, sheinpub.SubmissionPhaseConfirmRemote, response, remoteErr, time.Now())
	appendSheinSubmissionEvent(pkg, event)
	task.Result.UpdatedAt = time.Now()
	if s.saveTaskResult == nil {
		return nil
	}
	return s.saveTaskResult(ctx, taskID, task.Result)
}

func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshSuccess(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse) (*SheinRefreshRemoteStatusResult, error) {
	response = sheinpub.ConfirmedSubmissionResponse(response, action)
	_, event := completeSheinSubmitAttemptAndBuildEvent(pkg, taskID, action, requestID, response, nil, sheinpub.SubmissionStartedAt(pkg, action, requestID, time.Now()), time.Now())
	appendSheinSubmissionEvent(pkg, event)
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if s.persistSuccessfulSheinSubmission != nil {
		if err := s.persistSuccessfulSheinSubmission(ctx, taskID, task, action); err != nil {
			return nil, err
		}
	}

	selection := sheinpub.ResolveSubmissionRemoteRefreshSelection(pkg, action, requestID, time.Now())
	return &SheinRefreshRemoteStatusResult{
		TaskID:       taskID,
		Action:       action,
		RequestID:    requestID,
		RemoteStatus: selection.RemoteStatus,
	}, nil
}

func (s *taskTemporalSubmissionPersistenceService) recordTemporalFailureState(ctx context.Context, in submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error {
	if s.recordSheinSubmissionFailureForState == nil {
		return nil
	}
	return s.recordSheinSubmissionFailureForState(ctx, in.TaskID, in.Result, in.Package, in.Action, in.RequestID, in.Phase, in.Err)
}
