package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/listingkit/core"
)

func (s *service) loadTaskExecutionContext(ctx context.Context, taskID string) (context.Context, *Task, error) {
	task, err := s.repo.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return ctx, nil, err
	}
	ctx = WithTenantID(ctx, task.TenantID)
	userID := ""
	if task.Request != nil {
		userID = strings.TrimSpace(task.Request.UserID)
	}
	ctx = WithRequestIdentity(ctx, RequestIdentity{
		TenantID: task.TenantID,
		UserID:   userID,
	})
	return withSheinTaskStoreAccess(ctx, task), task, nil
}

func (s *service) markTaskProcessingIfPending(ctx context.Context, task *Task) error {
	if task == nil || task.Status != core.TaskStatusPending {
		return nil
	}
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, core.ErrTaskNotPending) {
			return nil
		}
		return fmt.Errorf("failed to mark task as processing: %w", err)
	}
	task.Status = core.TaskStatusProcessing
	return nil
}

func standardSnapshotFromTask(task *Task) (*StandardProductSnapshot, error) {
	if task == nil || task.Result == nil {
		return nil, fmt.Errorf("standard product snapshot is required before platform adaptation")
	}
	if task.Result.StandardProductSnapshot != nil {
		if task.Result.StandardProductSnapshot.CatalogProduct == nil {
			return nil, fmt.Errorf("product snapshot is required before platform adaptation")
		}
		return task.Result.StandardProductSnapshot, nil
	}
	return nil, fmt.Errorf("standard product snapshot is required before platform adaptation")
}

func (s *service) persistProcessedTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	if result == nil {
		return fmt.Errorf("listing kit result is nil")
	}
	if result.Summary != nil && result.Summary.NeedsReview {
		result.Status = string(core.TaskStatusNeedsReview)
		result.ReviewReasons = reviewReasonsFromResult(result)
		return s.repo.MarkNeedsReview(ctx, taskID, result, taskNeedsReviewReason(result))
	}
	result.Status = string(core.TaskStatusCompleted)
	return s.repo.MarkCompleted(ctx, taskID, result)
}
