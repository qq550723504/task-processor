package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingkit/core"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.Task == nil || state.completion.Package == nil {
		return nil, ErrTaskResultUnavailable
	}
	sheinpub.AppendSubmissionEvent(state.completion.Package, sheinpub.BuildSubmissionPhaseEvent(state.completion.TaskID, state.completion.Action, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionStatusRunning, state.completion.RequestID, state.now, "恢复本地提交完成状态", nil))
	if state.selection.Record != nil {
		if _, err := persistSheinRemoteCompletionSuccess(ctx, &state.completion, state.completion.Response, s.rememberSheinSubmitted, s.persistSuccessfulSubmission); err != nil {
			return nil, err
		}
		return s.buildTaskPreview(ctx, state.completion.Task, "shein")
	}
	return s.finalizeRecoveredSheinSubmission(ctx, state.completion.Task, state.completion.Action)
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.Task == nil || state.completion.Package == nil {
		return nil, ErrTaskResultUnavailable
	}
	if state.selection.Record == nil || strings.TrimSpace(state.selection.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", core.ErrSubmitInProgress)
	}
	return s.remoteRefreshRunner.Refresh(ctx, state)
}

func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {
	if state == nil || state.completion.Response == nil {
		return false
	}
	return sheinpub.RemoteSubmissionResponseAccepted(state.completion.Action, state.completion.Response)
}

func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {
	var result *ListingKitResult
	var pkg *SheinPackage
	if in.task != nil {
		result = in.task.Result
		if in.task.Result != nil {
			pkg = in.task.Result.Shein
		}
	}
	if s.recordSubmissionFailure == nil {
		return in.startErr
	}
	return s.recordSubmissionFailure(
		ctx,
		in.taskID,
		result,
		pkg,
		in.action,
		in.requestID,
		sheinpub.SubmissionPhaseValidate,
		in.startErr,
	)
}
