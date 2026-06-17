package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/core"
	sheinmarketpub "task-processor/internal/marketplace/shein/publishing"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, sheinpub.BuildSubmissionPhaseEvent(state.completion.taskID, state.completion.action, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionStatusRunning, state.completion.requestID, state.now, "恢复本地提交完成状态", nil))
	if state.selection.Record != nil {
		if _, err := persistSheinRemoteCompletionSuccess(ctx, &state.completion, state.completion.response, s.rememberSheinSubmitted, s.persistSuccessfulSubmission); err != nil {
			return nil, err
		}
		return s.buildTaskPreview(ctx, state.completion.task, "shein")
	}
	return s.finalizeRecoveredSheinSubmission(ctx, state.completion.task, state.completion.action)
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	if state.selection.Record == nil || strings.TrimSpace(state.selection.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", core.ErrSubmitInProgress)
	}
	return s.remoteRefreshRunner.Refresh(ctx, state)
}

func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {
	if state == nil || state.completion.response == nil {
		return false
	}
	return sheinmarketpub.ResponseAcceptedForAction(
		state.completion.action,
		state.completion.response.Success,
		state.completion.response.Code,
	)
}

func (s *taskSubmissionRecoveryService) beginSheinSubmitLeaseWithoutStartTime(ctx context.Context, taskID, action, requestID string) (*Task, error) {
	startedAt, _ := ctx.Value(sheinSubmitStartedAtContextKey{}).(time.Time)
	return s.beginSheinSubmitLease(ctx, taskID, action, requestID, startedAt)
}

func (s *taskSubmissionRecoveryService) buildSheinReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionRecoveryService) buildMissingPackageAcquireError(error) error {
	return fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
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

func (s *taskSubmissionRecoveryService) clearWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {
	return s.clearSheinSubmitLeaseAfterStartFailure(ctx, in.taskID, in.action, in.requestID, in.startErr)
}
