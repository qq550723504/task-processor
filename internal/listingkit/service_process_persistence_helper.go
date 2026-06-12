package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func deriveProcessTerminalStatus(result *ListingKitResult) TaskStatus {
	if resultRequiresTerminalReview(result) {
		return TaskStatusNeedsReview
	}
	return TaskStatusCompleted
}

func resultRequiresTerminalReview(result *ListingKitResult) bool {
	if result == nil {
		return false
	}
	if result.Summary != nil && result.Summary.NeedsReview {
		return true
	}
	if result.PodExecution != nil && result.PodExecution.Status == podStatusFailedBlocking {
		return true
	}
	return false
}

func applyProcessTerminalResult(result *ListingKitResult, status TaskStatus) *ListingKitResult {
	if result == nil {
		return nil
	}
	result.Status = string(status)
	if status == TaskStatusNeedsReview {
		result.ReviewReasons = reviewReasonsFromResult(result)
	}
	return result
}

func (s *service) persistProcessFailure(ctx context.Context, taskID string, result *ListingKitResult, err error) error {
	var persistErrors []error
	if result != nil {
		if saveErr := s.repo.SaveTaskResult(ctx, taskID, result); saveErr != nil {
			persistErrors = append(persistErrors, fmt.Errorf("save partial result: %w", saveErr))
		}
	}
	if persistErr := persistClassifiedTaskFailure(ctx, s.repo, taskID, err.Error(), err); persistErr != nil {
		persistErrors = append(persistErrors, fmt.Errorf("persist failure state: %w", persistErr))
	}
	return errors.Join(persistErrors...)
}

func (s *service) persistProcessSuccess(ctx context.Context, taskID string, result *ListingKitResult) error {
	switch deriveProcessTerminalStatus(result) {
	case TaskStatusNeedsReview:
		result = applyProcessTerminalResult(result, TaskStatusNeedsReview)
		return s.repo.MarkNeedsReview(ctx, taskID, result, taskNeedsReviewReason(result))
	default:
		result = applyProcessTerminalResult(result, TaskStatusCompleted)
		return s.repo.MarkCompleted(ctx, taskID, result)
	}
}

func taskNeedsReviewReason(result *ListingKitResult) string {
	warnings := reviewReasonsFromResult(result)
	if len(warnings) == 0 {
		return "listing kit requires review"
	}
	return strings.Join(warnings, "; ")
}
