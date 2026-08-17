package listingkit

import (
	"context"
	"errors"
	"fmt"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingsubscription"

	"github.com/sirupsen/logrus"
)

type listingKitProcessFlow struct {
	service                *service
	startUsageLeaseRenewal func(context.Context, *Task) (context.Context, func() error)
}

func buildListingKitProcessFlow(s *service) *listingKitProcessFlow {
	return &listingKitProcessFlow{
		service:                s,
		startUsageLeaseRenewal: s.startGenerationUsageReservationLeaseRenewal,
	}
}

func (f *listingKitProcessFlow) run(ctx context.Context, task *Task, log *logrus.Entry) (*ListingKitResult, error) {
	if err := f.claimTask(ctx, task); err != nil {
		return nil, err
	}
	log.Info("marked listing kit task as processing")
	reservation, usageEnabled, err := f.service.reserveGenerationUsage(ctx, task)
	if err != nil {
		var postReserveErr *generationUsagePostReservePersistenceError
		if errors.As(err, &postReserveErr) {
			if persistErr := f.service.persistGenerationUsageReconciliation(ctx, task, err); persistErr != nil {
				return nil, errors.Join(err, persistErr)
			}
			return nil, err
		}
		var replayReservationErr *generationUsageReplayReservationError
		if errors.As(err, &replayReservationErr) {
			if persistErr := f.service.persistGenerationUsageReconciliation(ctx, task, err); persistErr != nil {
				return nil, errors.Join(err, persistErr)
			}
			return nil, err
		}
		if errors.Is(err, listingsubscription.ErrUsageQuotaExceeded) {
			quotaErr := generationQuotaFailure(task.ID)
			if persistErr := markFailedTaskState(ctx, f.service.repo, task.ID, quotaErr.Error()); persistErr != nil {
				fallbackErr := markTerminalPersistencePending(ctx, f.service.repo, task.ID, persistErr)
				return nil, errors.Join(err, quotaErr, persistErr, fallbackErr)
			}
			return nil, err
		}
		if persistErr := f.service.persistReservationFailure(ctx, task, err); persistErr != nil {
			return nil, errors.Join(err, persistErr)
		}
		return nil, err
	}
	if usageEnabled && reservation.AlreadyCommitted {
		result, replayErr := generationUsageCommittedReplayResult(task)
		if replayErr != nil {
			return nil, replayErr
		}
		// claimTask moves the row to processing before the idempotent usage
		// lookup. Re-persist the already terminal result so a committed replay
		// cannot strand the task in processing.
		if persistErr := f.service.persistProcessSuccess(ctx, task.ID, result); persistErr != nil {
			fallbackErr := markCommittedReplayPersistencePending(ctx, f.service.repo, task.ID, persistErr)
			return nil, errors.Join(persistErr, fallbackErr)
		}
		if clearErr := f.service.clearGenerationUsageReservation(ctx, task); clearErr != nil {
			return nil, clearErr
		}
		return result, nil
	}

	workflowCtx := ctx
	stopLeaseRenewal := func() error { return nil }
	if usageEnabled {
		startLeaseRenewal := f.startUsageLeaseRenewal
		if startLeaseRenewal == nil {
			startLeaseRenewal = f.service.startGenerationUsageReservationLeaseRenewal
		}
		workflowCtx, stopLeaseRenewal = startLeaseRenewal(ctx, task)
	}
	result, workflowErr := f.service.runWorkflow(workflowCtx, task)
	if renewalErr := stopLeaseRenewal(); renewalErr != nil {
		// A failed lease renewal makes the provider outcome ambiguous: it may
		// have completed after its task-side lease stopped being durable. Keep
		// the reservation for operator reconciliation instead of releasing it
		// or scheduling another provider invocation.
		cause := errors.Join(workflowErr, renewalErr)
		if persistErr := f.service.persistGenerationUsageReconciliation(ctx, task, cause); persistErr != nil {
			return nil, errors.Join(cause, persistErr)
		}
		return nil, cause
	}
	if workflowErr != nil {
		log.WithError(workflowErr).Error("listing kit workflow failed")
		if _, retryable := classifyRetryableTaskFailure(workflowErr); retryable {
			var persistErr error
			if usageEnabled {
				persistErr = f.service.persistProcessRetryableFailure(ctx, task, result, workflowErr, true)
			} else {
				persistErr = f.service.persistProcessFailure(ctx, task.ID, result, workflowErr)
			}
			if persistErr != nil {
				log.WithError(persistErr).Error("failed to persist scheduled listing kit workflow failure")
				return nil, errors.Join(workflowErr, persistErr)
			}
			return nil, workflowErr
		}
		if usageEnabled {
			if releaseErr := f.service.releaseGenerationUsage(ctx, task, "workflow_failed"); releaseErr != nil {
				log.WithError(releaseErr).Error("failed to release listing kit generation usage")
				workflowErr = errors.Join(workflowErr, releaseErr)
				persistCtx, cancel := settlementPersistenceContext(ctx)
				if result != nil {
					if saveErr := f.service.repo.SaveTaskResult(persistCtx, task.ID, result); saveErr != nil {
						workflowErr = errors.Join(workflowErr, saveErr)
					}
				}
				cancel()
				if blockErr := f.service.markGenerationUsageReleasePending(ctx, task, "workflow_failed", workflowErr, releaseErr); blockErr != nil {
					workflowErr = errors.Join(workflowErr, blockErr)
				}
				return nil, workflowErr
			}
		}
		if persistErr := f.service.persistProcessFailure(ctx, task.ID, result, workflowErr); persistErr != nil {
			log.WithError(persistErr).Error("failed to persist listing kit workflow failure")
			// The ledger release has already succeeded and cleared the task-side
			// intent. If durable failure persistence then fails, retain an explicit
			// recovery block rather than leaving the claimed task in processing.
			fallbackErr := markTerminalPersistencePending(ctx, f.service.repo, task.ID, persistErr)
			return nil, errors.Join(workflowErr, persistErr, fallbackErr)
		}
		return nil, workflowErr
	}
	log.WithFields(logrus.Fields{
		"needs_review":  result != nil && result.Summary != nil && result.Summary.NeedsReview,
		"warning_count": processWarningCount(result),
	}).Info("listing kit workflow returned result")

	status := deriveProcessTerminalStatus(result)
	result = applyProcessTerminalResult(result, status)
	if status == core.TaskStatusNeedsReview {
		log.WithField("review_reason_count", len(result.ReviewReasons)).Info("marking listing kit task as needs_review")
		if err := f.service.persistProcessSuccess(ctx, task.ID, result); err != nil {
			log.WithError(err).Error("failed to mark listing kit task as needs_review")
			if !usageEnabled {
				return nil, err
			}
			return nil, f.service.handleGenerationTerminalPersistenceFailure(ctx, task, err)
		}
		if usageEnabled {
			if err := f.service.commitGenerationUsage(ctx, task); err != nil {
				return nil, f.service.markGenerationUsageCommitPending(ctx, task, err)
			}
		}
		log.Info("marked listing kit task as needs_review")
		return result, nil
	}

	log.Info("marking listing kit task as completed")
	if err := f.service.persistProcessSuccess(ctx, task.ID, result); err != nil {
		log.WithError(err).Error("failed to mark listing kit task as completed")
		if !usageEnabled {
			return nil, err
		}
		return nil, f.service.handleGenerationTerminalPersistenceFailure(ctx, task, err)
	}
	if usageEnabled {
		if err := f.service.commitGenerationUsage(ctx, task); err != nil {
			return nil, f.service.markGenerationUsageCommitPending(ctx, task, err)
		}
	}
	log.Info("marked listing kit task as completed")
	return result, nil
}

func (f *listingKitProcessFlow) claimTask(ctx context.Context, task *Task) error {
	if err := f.service.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, core.ErrTaskNotPending) {
			return core.ErrTaskNotPending
		}
		return fmt.Errorf("failed to mark task as processing: %w", err)
	}
	return nil
}

func processWarningCount(result *ListingKitResult) int {
	if result == nil || result.Summary == nil {
		return 0
	}
	return result.Summary.WarningCount
}
