package listingkit

import (
	"context"
	"strings"
	"time"

	listingsubmission "task-processor/internal/listing/submission"
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
		sheinpub.AppendSubmissionEvent(pkg, buildRecoverRemoteLeaseEvent(taskID, action, pkg.SubmissionState.CurrentPhase, requestID, startedAt))
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
	pkg, ok := sheinpub.PreviewPayloadPackage(task.Result.Shein)
	if !ok {
		return nil, errSheinSubmitMissingPackage
	}
	return pkg, nil
}

func shouldReplayExistingSheinSubmitLease(pkg *SheinPackage, action, requestID string) bool {
	return sheinpub.FindCompletedSubmissionRecordByRequestID(pkg, action, requestID) != nil
}

func shouldRecoverSheinSubmitLeaseWithSupplierCode(pkg *SheinPackage, action, requestID string, startedAt time.Time) bool {
	if !sheinpub.SubmissionLeaseNeedsRemoteRecovery(pkg, action, requestID, startedAt, sheinSubmitInFlightTTL) {
		return false
	}
	record := sheinpub.SubmissionRecordForAction(pkg.SubmissionState, action)
	return record != nil && strings.TrimSpace(record.SupplierCode) != ""
}

func buildRecoverRemoteLeaseEvent(taskID, action, phase, requestID string, startedAt time.Time) sheinpub.SubmissionEvent {
	return sheinpub.BuildSubmissionPhaseEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, startedAt, "远端可能已收到，正在刷新诊断状态", nil)
}

func validateActiveSheinSubmitLease(pkg *SheinPackage, action, requestID string, startedAt time.Time) error {
	active := sheinpub.FindActiveSubmissionAttempt(pkg, action, startedAt, sheinSubmitInFlightTTL)
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
	return listingsubmission.NewSubmitInProgressError("shein", action, active.CurrentPhase, active.CurrentRequestID, active.LeaseExpiresAt)
}

func beginNewSheinSubmitLease(task *Task, pkg *SheinPackage, taskID, action, requestID string, startedAt time.Time) {
	if task == nil || task.Result == nil || pkg == nil {
		return
	}
	sheinpub.BeginSubmitAttempt(pkg, action, requestID, sheinpub.SubmissionPhaseValidate, startedAt, sheinSubmitInFlightTTL)
	event := sheinpub.BuildSubmissionPhaseEvent(taskID, action, sheinpub.SubmissionPhaseValidate, sheinpub.SubmissionStatusRunning, requestID, startedAt, "", nil)
	sheinpub.AppendSubmissionEvent(pkg, event)
	task.Result.UpdatedAt = startedAt
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
	pkg, ok := sheinpub.SubmissionStatePackage(task.Result.Shein)
	if !ok {
		return nil
	}
	return pkg
}

func clearSheinSubmitLeaseState(task *Task, pkg *SheinPackage, action, requestID string) {
	if task == nil || pkg == nil || pkg.SubmissionState == nil {
		return
	}
	sheinpub.ClearSubmissionInFlight(pkg.SubmissionState, action, requestID)
	task.Result.UpdatedAt = time.Now()
}

func markSheinSubmitStartFailure(pkg *SheinPackage, taskID, action, requestID string, startErr error) {
	record := sheinpub.ApplySubmissionStartFailure(pkg, action, requestID, startErr, time.Now())
	if record == nil {
		return
	}
	sheinpub.AppendSubmissionEvent(pkg, sheinpub.BuildSubmissionAttemptEvent(taskID, action, record, nil, startErr, record.StartedAt))
}
