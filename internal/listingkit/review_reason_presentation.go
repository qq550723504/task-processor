package listingkit

import "strings"

func summarizeReviewReasons(reasons []string, fallback string) string {
	if len(reasons) == 0 {
		return fallback
	}
	return strings.Join(reasons, "; ")
}

func buildTaskResultReviewState(task *Task, resultPayload *ListingKitResult) ([]string, string) {
	if task == nil {
		return nil, ""
	}

	reviewReasons := reviewReasonsFromTask(task)
	if resultPayload != nil {
		if reasons := reviewReasonsFromResult(resultPayload); len(reasons) > 0 {
			reviewReasons = reasons
		}
	}

	effectiveError := task.Error
	if task.Status == TaskStatusNeedsReview {
		effectiveError = summarizeReviewReasons(reviewReasons, effectiveError)
	}
	return reviewReasons, effectiveError
}
