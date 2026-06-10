package listingkit

import (
	"context"
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskSubmissionRecoveryService) beginSheinSubmitLease(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, error) {
	return s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		pkg, err := loadSheinSubmitLeasePackage(task)
		if err != nil {
			return err
		}
		if err := s.prepareSheinSubmitLease(task, pkg, taskID, action, requestID, startedAt); err != nil {
			return err
		}
		return nil
	})
}

func (s *taskSubmissionRecoveryService) prepareSheinSubmitLease(task *Task, pkg *SheinPackage, taskID, action, requestID string, startedAt time.Time) error {
	if shouldReplayExistingSheinSubmitLease(pkg, action, requestID) {
		return errSheinSubmitReplayExisting
	}
	if shouldRecoverSheinSubmitLeaseWithSupplierCode(pkg, action, requestID, startedAt) {
		appendSheinSubmissionEvent(pkg, buildRecoverRemoteLeaseEvent(taskID, action, pkg.SubmissionState.CurrentPhase, requestID, startedAt))
		return errSheinSubmitRecoverRemote
	}
	if err := validateActiveSheinSubmitLease(pkg, action, requestID, startedAt); err != nil {
		return err
	}
	beginNewSheinSubmitLease(task, pkg, taskID, action, requestID, startedAt)
	return nil
}

func loadSheinSubmitLeasePackage(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.PreviewPayload == nil {
		return nil, errSheinSubmitMissingPackage
	}
	return pkg, nil
}

func shouldReplayExistingSheinSubmitLease(pkg *SheinPackage, action, requestID string) bool {
	return findSheinSubmissionRecordByRequestID(pkg, action, requestID) != nil
}

func shouldRecoverSheinSubmitLeaseWithSupplierCode(pkg *SheinPackage, action, requestID string, startedAt time.Time) bool {
	if !shouldRecoverSheinSubmitLeaseRemote(pkg, action, requestID, startedAt) {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	return record != nil && strings.TrimSpace(record.SupplierCode) != ""
}

func shouldRecoverSheinSubmitLeaseRemote(pkg *SheinPackage, action, requestID string, startedAt time.Time) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	sameRequestNeedsRecovery := pkg.SubmissionState.CurrentRequestID == requestID &&
		(pkg.SubmissionState.CurrentPhase != sheinpub.SubmissionPhaseSubmitRemote ||
			sheinSubmitRemoteResponsePersisted(pkg, action, requestID))
	return sameRequestNeedsRecovery || sheinSubmitAttemptNeedsRemoteRecovery(pkg.SubmissionState, action, startedAt)
}

func buildRecoverRemoteLeaseEvent(taskID, action, phase, requestID string, startedAt time.Time) sheinpub.SubmissionEvent {
	return submission.BuildPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, startedAt, "远端可能已收到，正在刷新诊断状态", nil)
}

func validateActiveSheinSubmitLease(pkg *SheinPackage, action, requestID string, startedAt time.Time) error {
	active := findActiveSheinSubmitAttempt(pkg, action, startedAt)
	if active == nil {
		return nil
	}
	if active.CurrentRequestID == requestID {
		return errSheinSubmitReplayExisting
	}
	return buildSheinSubmitInProgressError(action, active)
}

func buildSheinSubmitInProgressError(action string, active *sheinpub.SubmissionReport) error {
	if active == nil {
		return nil
	}
	return &submission.SubmitInProgressError{
		Platform:       "shein",
		Action:         action,
		Phase:          active.CurrentPhase,
		RequestID:      active.CurrentRequestID,
		LeaseExpiresAt: active.LeaseExpiresAt,
	}
}

func beginNewSheinSubmitLease(task *Task, pkg *SheinPackage, taskID, action, requestID string, startedAt time.Time) {
	if task == nil || task.Result == nil || pkg == nil {
		return
	}
	_, event := submission.BeginAttemptAndBuildEvent(pkg, taskID, action, requestID, sheinpub.SubmissionPhaseValidate, startedAt, sheinSubmitInFlightTTL)
	appendSheinSubmissionEvent(pkg, event)
	task.Result.UpdatedAt = startedAt
}

func sheinSubmitRemoteResponsePersisted(pkg *SheinPackage, action, requestID string) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID {
		return false
	}
	return record.Result != nil
}

func (s *taskSubmissionRecoveryService) clearSheinSubmitLease(ctx context.Context, taskID, action, requestID string) error {
	return s.mutateSheinSubmitLease(ctx, taskID, func(task *Task, pkg *SheinPackage) {
		clearSheinSubmitLeaseState(task, pkg, action, requestID)
	})
}

func (s *taskSubmissionRecoveryService) clearSheinSubmitLeaseAfterStartFailure(ctx context.Context, taskID, action, requestID string, startErr error) error {
	return s.mutateSheinSubmitLease(ctx, taskID, func(task *Task, pkg *SheinPackage) {
		markSheinSubmitStartFailure(pkg, taskID, action, requestID, startErr)
		clearSheinSubmitLeaseState(task, pkg, action, requestID)
	})
}

func (s *taskSubmissionRecoveryService) mutateSheinSubmitLease(ctx context.Context, taskID string, mutate func(*Task, *SheinPackage)) error {
	_, err := s.mutateTaskResult(ctx, taskID, func(task *Task) error {
		pkg := loadSheinSubmitLeaseState(task)
		if pkg == nil {
			return nil
		}
		if mutate != nil {
			mutate(task, pkg)
		}
		return nil
	})
	return err
}

func loadSheinSubmitLeaseState(task *Task) *SheinPackage {
	if task == nil || task.Result == nil {
		return nil
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil
	}
	return pkg
}

func clearSheinSubmitLeaseState(task *Task, pkg *SheinPackage, action, requestID string) {
	if task == nil || pkg == nil || pkg.SubmissionState == nil {
		return
	}
	clearSheinSubmitInFlight(pkg.SubmissionState, action, requestID)
	task.Result.UpdatedAt = time.Now()
}

func markSheinSubmitStartFailure(pkg *SheinPackage, taskID, action, requestID string, startErr error) {
	if pkg == nil || pkg.SubmissionState == nil {
		return
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || record.RequestID != requestID || record.Status != sheinpub.SubmissionStatusRunning {
		return
	}
	finishedAt := time.Now()
	record.Status = sheinpub.SubmissionStatusFailed
	record.Phase = sheinpub.SubmissionPhaseValidate
	record.FinishedAt = &finishedAt
	if startErr != nil {
		record.Error = startErr.Error()
	}
	submission.ApplyRecord(pkg, record)
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(taskID, action, record, nil, startErr, record.StartedAt))
}
