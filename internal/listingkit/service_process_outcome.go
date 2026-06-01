package listingkit

import "context"

func deriveProcessTerminalStatus(result *ListingKitResult) TaskStatus {
	if result != nil && result.Summary != nil && result.Summary.NeedsReview {
		return TaskStatusNeedsReview
	}
	return TaskStatusCompleted
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

func (s *service) persistProcessFailure(ctx context.Context, taskID string, result *ListingKitResult, err error) {
	if result != nil {
		_ = s.repo.SaveTaskResult(ctx, taskID, result)
	}
	_ = s.repo.MarkFailed(ctx, taskID, err.Error())
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
