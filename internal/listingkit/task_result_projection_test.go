package listingkit

import (
	"testing"
	"time"
)

func TestBuildTaskResultProjectionIncludesTerminalLifecycleAndReviewState(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	task := &Task{
		Status:    TaskStatusNeedsReview,
		Error:     "legacy error",
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
	}
	resultPayload := &ListingKitResult{
		ReviewReasons: []string{"payload reason"},
	}

	projection := buildTaskResultProjection(task, resultPayload)
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if projection.Lifecycle.Status != TaskStatusNeedsReview || projection.Lifecycle.Error != "payload reason" {
		t.Fatalf("Lifecycle = %+v", projection.Lifecycle)
	}
	if projection.Lifecycle.CompletedAt == nil || !projection.Lifecycle.CompletedAt.Equal(now) {
		t.Fatalf("CompletedAt = %v, want %v", projection.Lifecycle.CompletedAt, now)
	}
	if len(projection.ReviewReasons) != 1 || projection.ReviewReasons[0] != "payload reason" {
		t.Fatalf("ReviewReasons = %#v", projection.ReviewReasons)
	}
}
