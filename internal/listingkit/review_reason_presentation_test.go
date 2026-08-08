package listingkit

import (
	"task-processor/internal/listingkit/core"
	"testing"
)

func TestSummarizeReviewReasonsUsesFallbackWhenEmpty(t *testing.T) {
	t.Parallel()

	if got := summarizeReviewReasons(nil, "fallback"); got != "fallback" {
		t.Fatalf("summarizeReviewReasons() = %q, want fallback", got)
	}
}

func TestBuildTaskResultReviewStatePrefersPayloadReasonsForNeedsReview(t *testing.T) {
	t.Parallel()

	task := &Task{
		Status: core.TaskStatusNeedsReview,
		Error:  "legacy error",
		Result: &ListingKitResult{
			ReviewReasons: []string{"task reason"},
		},
	}
	resultPayload := &ListingKitResult{
		ReviewReasons: []string{"payload reason"},
	}

	reviewReasons, effectiveError := buildTaskResultReviewState(task, resultPayload)
	if len(reviewReasons) != 1 || reviewReasons[0] != "payload reason" {
		t.Fatalf("reviewReasons = %#v, want payload reason", reviewReasons)
	}
	if effectiveError != "payload reason" {
		t.Fatalf("effectiveError = %q, want payload summary", effectiveError)
	}
}
