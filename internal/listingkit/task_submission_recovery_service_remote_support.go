package listingkit

import (
	"context"
	sheinpub "task-processor/internal/publishing/shein"
	"time"
)

func loadRecoveredSheinSubmissionReport(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg, ok := sheinpub.SubmissionStatePackage(task.Result.Shein)
	if !ok {
		return nil, ErrTaskResultUnavailable
	}
	return pkg, nil
}

func buildRecoveredSheinRemoteState(task *Task, action string) (*sheinRecoveredRemoteState, error) {
	pkg, err := loadRecoveredSheinSubmissionReport(task)
	if err != nil {
		return nil, err
	}
	selection := sheinpub.ResolveSubmissionRecoverySelection(pkg, action)
	if selection.Report == nil {
		return nil, ErrTaskResultUnavailable
	}
	taskID := ""
	if task != nil {
		taskID = task.ID
	}
	return &sheinRecoveredRemoteState{
		completion: sheinRemoteCompletionState{
			taskID:    taskID,
			task:      task,
			pkg:       pkg,
			action:    action,
			requestID: selection.RequestID,
			startedAt: selection.StartedAt,
			response:  selection.Response,
		},
		selection: selection,
		now:       time.Now(),
	}, nil
}

func buildRecoveredSheinRemoteRefreshState(state *sheinRecoveredRemoteState) *sheinRemoteRefreshExecutionState {
	if state == nil {
		return nil
	}
	return newSheinRemoteRefreshExecutionState(state.completion, state.selection.SupplierCode, state.now)
}

func (s *taskSubmissionRecoveryService) persistSheinRecoveredRemoteFailure(ctx context.Context, state *sheinRecoveredRemoteState, remoteErr error) error {
	if state == nil {
		return remoteErr
	}
	return persistSheinRemoteCompletionFailure(ctx, s.repo.SaveTaskResult, &state.completion, sheinpub.SubmissionPhaseConfirmRemote, remoteErr)
}

func (s *taskSubmissionRecoveryService) completeSheinRecoveredRemoteSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	record, err := persistSheinRemoteCompletionSuccess(ctx, &state.completion, state.completion.response, s.rememberSheinSubmitted, s.persistSuccessfulSubmission)
	if err != nil {
		return nil, err
	}
	if state.selection.Record != nil && record != nil && record.Result == nil {
		record.Status = sheinpub.SubmissionStatusSuccess
	}
	return s.buildTaskPreview(ctx, state.completion.task, "shein")
}

func (s *taskSubmissionRecoveryService) finalizeRecoveredSheinSubmission(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if s.persistSuccessfulSubmission != nil {
		if err := s.persistSuccessfulSubmission(ctx, task.ID, task, action); err != nil {
			return nil, err
		}
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionRecoveryService) persistRecoveredRemoteRefreshPhase(_ context.Context, state *sheinRecoveredRemoteState) error {
	if state == nil || state.completion.pkg == nil {
		return ErrTaskResultUnavailable
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, advanceSheinSubmitPhaseAndBuildEvent(state.completion.pkg, state.completion.taskID, state.completion.action, state.completion.requestID, sheinpub.SubmissionPhaseConfirmRemote, state.now))
	return nil
}

func (s *taskSubmissionRecoveryService) buildRecoveredRemoteRefreshRequest(ctx context.Context, state *sheinRecoveredRemoteState) (*sheinRemoteRefreshRequest, error) {
	if state == nil || state.completion.task == nil {
		return nil, ErrTaskResultUnavailable
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, state.completion.task)
	if err != nil {
		return nil, err
	}
	return buildSheinRemoteRefreshRequest(productAPI, buildRecoveredSheinRemoteRefreshState(state)), nil
}

func (s *taskSubmissionRecoveryService) recordRecoveredRemoteRefreshEvent(state *sheinRecoveredRemoteState, event *sheinpub.SubmissionEvent) {
	if state == nil || state.completion.pkg == nil || event == nil {
		return
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, *event)
}

func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshError(ctx context.Context, state *sheinRecoveredRemoteState, remoteErr error) (*ListingKitPreview, error) {
	return nil, s.persistSheinRecoveredRemoteFailure(ctx, state, remoteErr)
}

func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	return s.completeSheinRecoveredRemoteSuccess(ctx, state)
}
