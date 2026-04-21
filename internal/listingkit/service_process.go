package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, ErrTaskNotPending) {
			return nil, ErrTaskNotPending
		}
		return nil, fmt.Errorf("failed to mark task as processing: %w", err)
	}

	result, err := s.runWorkflow(ctx, task)
	if err != nil {
		if result != nil {
			_ = s.repo.SaveTaskResult(ctx, task.ID, result)
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}

	if result.Summary != nil && result.Summary.NeedsReview {
		result.Status = string(TaskStatusNeedsReview)
		if err := s.repo.MarkNeedsReview(ctx, task.ID, result, taskNeedsReviewReason(result)); err != nil {
			return nil, err
		}
		return result, nil
	}

	result.Status = string(TaskStatusCompleted)
	if err := s.repo.MarkCompleted(ctx, task.ID, result); err != nil {
		return nil, err
	}
	return result, nil
}

func taskNeedsReviewReason(result *ListingKitResult) string {
	if result == nil || result.Summary == nil {
		return "listing kit requires review"
	}
	warnings := make([]string, 0, len(result.Summary.Warnings))
	for _, warning := range result.Summary.Warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		warnings = append(warnings, warning)
	}
	if len(warnings) == 0 {
		return "listing kit requires review"
	}
	return strings.Join(warnings, "; ")
}
