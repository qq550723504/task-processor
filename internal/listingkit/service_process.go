package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/service_process",
		"task_id":   task.ID,
	})
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, ErrTaskNotPending) {
			return nil, ErrTaskNotPending
		}
		return nil, fmt.Errorf("failed to mark task as processing: %w", err)
	}
	log.Info("marked listing kit task as processing")

	result, err := s.runWorkflow(ctx, task)
	if err != nil {
		log.WithError(err).Error("listing kit workflow failed")
		if result != nil {
			_ = s.repo.SaveTaskResult(ctx, task.ID, result)
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}
	log.WithFields(logrus.Fields{
		"needs_review": result != nil && result.Summary != nil && result.Summary.NeedsReview,
		"warning_count": func() int {
			if result == nil || result.Summary == nil {
				return 0
			}
			return result.Summary.WarningCount
		}(),
	}).Info("listing kit workflow returned result")

	if result.Summary != nil && result.Summary.NeedsReview {
		result.Status = string(TaskStatusNeedsReview)
		result.ReviewReasons = reviewReasonsFromResult(result)
		log.WithField("review_reason_count", len(result.ReviewReasons)).Info("marking listing kit task as needs_review")
		if err := s.repo.MarkNeedsReview(ctx, task.ID, result, taskNeedsReviewReason(result)); err != nil {
			log.WithError(err).Error("failed to mark listing kit task as needs_review")
			return nil, err
		}
		log.Info("marked listing kit task as needs_review")
		return result, nil
	}

	result.Status = string(TaskStatusCompleted)
	log.Info("marking listing kit task as completed")
	if err := s.repo.MarkCompleted(ctx, task.ID, result); err != nil {
		log.WithError(err).Error("failed to mark listing kit task as completed")
		return nil, err
	}
	log.Info("marked listing kit task as completed")
	return result, nil
}

func taskNeedsReviewReason(result *ListingKitResult) string {
	warnings := reviewReasonsFromResult(result)
	if len(warnings) == 0 {
		return "listing kit requires review"
	}
	return strings.Join(warnings, "; ")
}
