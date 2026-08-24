package listingkit

import (
	"context"
	"errors"
	"fmt"
	"task-processor/internal/listingkit/core"
)

func deriveProcessTerminalStatus(result *ListingKitResult) core.TaskStatus {
	if resultRequiresTerminalReview(result) {
		return core.TaskStatusNeedsReview
	}
	return core.TaskStatusCompleted
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

func applyProcessTerminalResult(result *ListingKitResult, status core.TaskStatus) *ListingKitResult {
	if result == nil {
		return nil
	}
	result.Status = string(status)
	if status == core.TaskStatusNeedsReview {
		result.ReviewReasons = reviewReasonsFromResult(result)
	} else {
		result.ReviewReasons = nil
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
	case core.TaskStatusNeedsReview:
		result = applyProcessTerminalResult(result, core.TaskStatusNeedsReview)
		return s.repo.MarkNeedsReview(ctx, taskID, result, taskNeedsReviewReason(result))
	default:
		result = applyProcessTerminalResult(result, core.TaskStatusCompleted)
		return s.repo.MarkCompleted(ctx, taskID, result)
	}
}

func taskNeedsReviewReason(result *ListingKitResult) string {
	return TaskNeedsReviewReason(result)
}

// TaskNeedsReviewReason derives the durable operator-facing reason from a
// terminal result. Storage and service code share this function so settlement
// recovery restores exactly the same reason as the original terminal write.
func TaskNeedsReviewReason(result *ListingKitResult) string {
	warnings := reviewReasonsFromResult(result)
	return summarizeReviewReasons(warnings, "listing kit requires review")
}
