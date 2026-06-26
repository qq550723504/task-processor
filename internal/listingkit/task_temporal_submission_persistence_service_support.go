package listingkit

import (
	"context"
	"errors"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskTemporalSubmissionPersistenceService) loadSheinSubmitPersistenceState(ctx context.Context, taskID, action, requestID, supplierCode string, response *sheinpub.SubmissionResponse, snapshot *sheinpub.SubmitSnapshot, phase, errorMessage string) (*sheinTemporalSubmissionPersistenceState, error) {
	if s == nil || s.loadSheinPublishTask == nil {
		return nil, errors.New("shein publish task loader is not configured")
	}
	task, pkg, err := s.loadSheinPublishTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	sheinpub.ApplySubmissionPersistenceInput(pkg, action, requestID, supplierCode, response, snapshot)
	return &sheinTemporalSubmissionPersistenceState{
		completion: sheinRemoteCompletionState{
			TaskID:    taskID,
			Task:      task,
			Package:   pkg,
			Action:    action,
			RequestID: requestID,
			Response:  response,
		},
		phase:        phase,
		errorMessage: errorMessage,
	}, nil
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
	if state == nil || state.completion.Task == nil || state.completion.Package == nil {
		return ErrTaskResultUnavailable
	}
	state.completion.StartedAt = sheinpub.SubmissionStartedAt(state.completion.Package, state.completion.Action, state.completion.RequestID, time.Now())
	return s.resultRunner.PersistSuccess(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    state.completion.TaskID,
		Task:      state.completion.Task,
		Result:    state.completion.Task.Result,
		Package:   state.completion.Package,
		Action:    state.completion.Action,
		RequestID: state.completion.RequestID,
		Response:  state.completion.Response,
		StartedAt: state.completion.StartedAt,
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
	_, event := sheinpub.CompleteSubmitAttemptAndBuildEvent(in.Package, in.TaskID, in.Action, in.RequestID, in.Response, nil, in.StartedAt, finishedAt)
	sheinpub.AppendSubmissionEvent(in.Package, event)
}

func (s *taskTemporalSubmissionPersistenceService) persistSheinTemporalSubmissionFailure(ctx context.Context, state *sheinTemporalSubmissionPersistenceState) error {
	if state == nil {
		return nil
	}
	var result *ListingKitResult
	if state.completion.Task != nil {
		result = state.completion.Task.Result
	}
	return s.resultRunner.PersistFailure(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		TaskID:    state.completion.TaskID,
		Result:    result,
		Package:   state.completion.Package,
		Action:    state.completion.Action,
		RequestID: state.completion.RequestID,
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
	return persistSheinRemoteCompletionFailure(ctx, s.saveTaskResult, &state.Completion, sheinpub.SubmissionPhaseConfirmRemote, remoteErr)
}

func (s *taskTemporalSubmissionPersistenceService) finishSheinTemporalRemoteRefreshSuccess(ctx context.Context, state *sheinTemporalRemoteRefreshState) (*SheinRefreshRemoteStatusResult, error) {
	if state == nil || state.Completion.Task == nil || state.Completion.Package == nil {
		return nil, ErrTaskResultUnavailable
	}
	response := sheinpub.ResolveConfirmedRemoteRefreshResponse(state.Completion.Response, state.Completion.Action)
	if _, err := persistSheinRemoteCompletionSuccess(ctx, &state.Completion, response, s.rememberSheinSubmitted, s.persistSuccessfulSheinSubmission); err != nil {
		return nil, err
	}

	remoteStatus := ""
	if pkg, ok := sheinpub.SubmissionStatePackage(state.Completion.Package); ok && pkg.SubmissionState != nil {
		remoteStatus = pkg.SubmissionState.RemoteStatus
	}
	return &SheinRefreshRemoteStatusResult{
		TaskID:       state.Completion.TaskID,
		Action:       state.Completion.Action,
		RequestID:    state.Completion.RequestID,
		RemoteStatus: remoteStatus,
	}, nil
}
