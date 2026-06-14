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
	resultRunner                         *submissiondomain.ResultPersistenceService[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]
}

type sheinTemporalSubmissionPersistenceState struct {
	completion   sheinRemoteCompletionState
	phase        string
	errorMessage string
}

func newTaskTemporalSubmissionPersistenceService(config taskTemporalSubmissionPersistenceServiceConfig) *taskTemporalSubmissionPersistenceService {
	service := &taskTemporalSubmissionPersistenceService{
		loadSheinPublishTask:                 config.loadSheinPublishTask,
		saveTaskResult:                       config.saveTaskResult,
		persistSheinSubmitPhase:              config.persistSheinSubmitPhase,
		persistSuccessfulSheinSubmission:     config.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: config.recordSheinSubmissionFailureForState,
		rememberSheinSubmitted:               config.rememberSheinSubmitted,
	}
	successRunner := config.successRunner
	if successRunner == nil {
		successRunner = newSheinSubmissionSuccessPersistenceService(
			service.completeTemporalSubmitAttempt,
			service.persistTemporalSuccessResultAndPhase,
			service.rememberSheinSubmitted,
			service.persistSuccessfulSheinSubmission,
		)
	}
	failureRunner := config.failureRunner
	if failureRunner == nil {
		failureRunner = newSheinSubmissionFailurePersistenceService(service.recordTemporalFailureState)
	}
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
		FallbackSuccess: func(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
			return service.persistTemporalSuccessFallback(ctx, in)
		},
		FallbackFailure: func(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
			return service.recordTemporalFailureState(ctx, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
				TaskID:    in.TaskID,
				Result:    in.Result,
				Package:   in.Package,
				Action:    in.Action,
				RequestID: in.RequestID,
				Phase:     in.Phase,
				Err:       in.Err,
			})
		},
	})
	return service
}

func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	state, err := s.loadSheinSubmitPersistenceState(ctx, in.TaskID, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot, "", "")
	if err != nil {
		return err
	}
	return s.persistSheinTemporalSubmissionSuccess(ctx, state)
}

func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	state, err := s.loadSheinSubmitPersistenceState(ctx, in.TaskID, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot, in.Phase, in.ErrorMessage)
	if err != nil {
		return err
	}
	return s.persistSheinTemporalSubmissionFailure(ctx, state)
}

func (s *taskTemporalSubmissionPersistenceService) loadSheinSubmitPersistenceState(ctx context.Context, taskID, action, requestID, supplierCode string, response *sheinpub.SubmissionResponse, snapshot *sheinpub.SubmitSnapshot, phase, errorMessage string) (*sheinTemporalSubmissionPersistenceState, error) {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sheinpub.ApplySubmissionPersistenceInput(pkg, action, requestID, supplierCode, response, snapshot)
	return &sheinTemporalSubmissionPersistenceState{
		completion: sheinRemoteCompletionState{
			taskID:    taskID,
			task:      task,
			pkg:       pkg,
			action:    action,
			requestID: requestID,
			response:  response,
		},
		phase:        phase,
		errorMessage: errorMessage,
	}, nil
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

func (s *taskTemporalSubmissionPersistenceService) persistSheinTemporalSubmissionSuccess(ctx context.Context, state *sheinTemporalSubmissionPersistenceState) error {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return ErrTaskResultUnavailable
	}
	state.completion.startedAt = sheinpub.SubmissionStartedAt(state.completion.pkg, state.completion.action, state.completion.requestID, time.Now())
	return s.resultRunner.PersistSuccess(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    state.completion.taskID,
		Task:      state.completion.task,
		Result:    state.completion.task.Result,
		Package:   state.completion.pkg,
		Action:    state.completion.action,
		RequestID: state.completion.requestID,
		Response:  state.completion.response,
		StartedAt: state.completion.startedAt,
	})
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
	sheinpub.AppendSubmissionEvent(in.Package, event)
}

func (s *taskTemporalSubmissionPersistenceService) persistSheinTemporalSubmissionFailure(ctx context.Context, state *sheinTemporalSubmissionPersistenceState) error {
	if state == nil {
		return nil
	}
	var result *ListingKitResult
	if state.completion.task != nil {
		result = state.completion.task.Result
	}
	return s.resultRunner.PersistFailure(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    state.completion.taskID,
		Result:    result,
		Package:   state.completion.pkg,
		Action:    state.completion.action,
		RequestID: state.completion.requestID,
		Phase:     state.phase,
		Err:       errors.New(strings.TrimSpace(state.errorMessage)),
	})
}

func (s *taskTemporalSubmissionPersistenceService) persistTemporalSuccessFallback(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
	if in.Task == nil || in.Package == nil {
		return ErrTaskResultUnavailable
	}
	in.Task.Result.UpdatedAt = time.Now()
	if s.saveTaskResult != nil {
		if err := s.saveTaskResult(ctx, in.TaskID, in.Task.Result); err != nil {
			return err
		}
	}
	if s.persistSheinSubmitPhase != nil {
		if err := s.persistSheinSubmitPhase(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, sheinpub.SubmissionPhasePersistResult); err != nil {
			return err
		}
	}
	s.completeTemporalSubmitAttempt(submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    in.TaskID,
		Task:      in.Task,
		Package:   in.Package,
		Action:    in.Action,
		RequestID: in.RequestID,
		Response:  in.Response,
		StartedAt: in.StartedAt,
	}, time.Now())
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(in.Task, in.Action)
	}
	return s.persistSuccessfulSheinSubmission(ctx, in.TaskID, in.Task, in.Action)
}

func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshFailure(ctx context.Context, state *sheinTemporalRemoteRefreshState, remoteErr error) error {
	if state == nil {
		return remoteErr
	}
	return persistSheinRemoteCompletionFailure(ctx, s.saveTaskResult, &state.completion, sheinpub.SubmissionPhaseConfirmRemote, remoteErr)
}

func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshSuccess(ctx context.Context, state *sheinTemporalRemoteRefreshState) (*SheinRefreshRemoteStatusResult, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	response := sheinpub.ConfirmedSubmissionResponse(state.completion.response, state.completion.action)
	if _, err := persistSheinRemoteCompletionSuccess(ctx, &state.completion, response, s.rememberSheinSubmitted, s.persistSuccessfulSheinSubmission); err != nil {
		return nil, err
	}

	selection := sheinpub.ResolveSubmissionRemoteRefreshSelection(state.completion.pkg, state.completion.action, state.completion.requestID, time.Now())
	return &SheinRefreshRemoteStatusResult{
		TaskID:       state.completion.taskID,
		Action:       state.completion.action,
		RequestID:    state.completion.requestID,
		RemoteStatus: selection.RemoteStatus,
	}, nil
}

func (s *taskTemporalSubmissionPersistenceService) recordTemporalFailureState(ctx context.Context, in submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error {
	if s.recordSheinSubmissionFailureForState == nil {
		return nil
	}
	return s.recordSheinSubmissionFailureForState(ctx, in.TaskID, in.Result, in.Package, in.Action, in.RequestID, in.Phase, in.Err)
}
