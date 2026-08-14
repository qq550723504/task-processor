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
		return nil, err
	}
	if usageEnabled && reservation.AlreadyCommitted {
		return generationUsageCommittedReplayResult(task)
	}

	result, err := f.service.runWorkflow(ctx, task)
	if err != nil {
		log.WithError(err).Error("listing kit workflow failed")
		if releaseErr := f.service.releaseGenerationUsage(ctx, task, "workflow_failed"); releaseErr != nil {
			log.WithError(releaseErr).Error("failed to release listing kit generation usage")
			err = errors.Join(err, releaseErr)
		}
		if persistErr := f.service.persistProcessFailure(ctx, task.ID, result, err); persistErr != nil {
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
			return nil, err
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
		return nil, err
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
