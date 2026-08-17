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
	service *service
}

func buildListingKitProcessFlow(s *service) *listingKitProcessFlow {
	return &listingKitProcessFlow{service: s}
}

func (f *listingKitProcessFlow) run(ctx context.Context, task *Task, log *logrus.Entry) (*ListingKitResult, error) {
	if err := f.claimTask(ctx, task); err != nil {
		return nil, err
	}
	log.Info("marked listing kit task as processing")
	reservation, usageEnabled, err := f.service.reserveGenerationUsage(ctx, task)
	if err != nil {
		if errors.Is(err, listingsubscription.ErrUsageQuotaExceeded) {
			quotaErr := generationQuotaFailure(task.ID)
			if persistErr := f.service.repo.MarkFailed(ctx, task.ID, quotaErr.Error()); persistErr != nil {
				return nil, errors.Join(quotaErr, persistErr)
			}
			return nil, err
		}
		if persistErr := f.service.persistProcessFailure(ctx, task.ID, nil, err); persistErr != nil {
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
			return nil, persistErr
		}
		return result, nil
	}

	result, err := f.service.runWorkflow(ctx, task)
	if err != nil {
		workflowErr := err
		log.WithError(err).Error("listing kit workflow failed")
		if _, retryable := classifyRetryableTaskFailure(err); !retryable {
			if releaseErr := f.service.releaseGenerationUsage(ctx, task, "workflow_failed"); releaseErr != nil {
				log.WithError(releaseErr).Error("failed to release listing kit generation usage")
				err = errors.Join(err, releaseErr)
				persistCtx, cancel := settlementPersistenceContext(ctx)
				if result != nil {
					if saveErr := f.service.repo.SaveTaskResult(persistCtx, task.ID, result); saveErr != nil {
						err = errors.Join(err, saveErr)
					}
				}
				cancel()
				if blockErr := f.service.markGenerationUsageReleasePending(ctx, task, workflowErr, releaseErr); blockErr != nil {
					err = errors.Join(err, blockErr)
				}
				return nil, err
			}
		}
		if persistErr := f.service.persistProcessFailure(ctx, task.ID, result, workflowErr); persistErr != nil {
			log.WithError(persistErr).Error("failed to persist listing kit workflow failure")
			return nil, errors.Join(err, persistErr)
		}
		return nil, err
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
			return nil, f.service.handleGenerationTerminalPersistenceFailure(ctx, task, err)
		}
		if err := f.service.commitGenerationUsage(ctx, task); err != nil {
			return nil, f.service.markGenerationUsageCommitPending(ctx, task, err)
		}
		log.Info("marked listing kit task as needs_review")
		return result, nil
	}

	log.Info("marking listing kit task as completed")
	if err := f.service.persistProcessSuccess(ctx, task.ID, result); err != nil {
		log.WithError(err).Error("failed to mark listing kit task as completed")
		return nil, f.service.handleGenerationTerminalPersistenceFailure(ctx, task, err)
	}
	if err := f.service.commitGenerationUsage(ctx, task); err != nil {
		return nil, f.service.markGenerationUsageCommitPending(ctx, task, err)
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
