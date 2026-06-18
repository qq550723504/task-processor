package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	assetpkg "task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
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
	return ctx, task, nil
}

func (s *service) markTaskProcessingIfPending(ctx context.Context, task *Task) error {
	if task == nil || task.Status != TaskStatusPending {
		return nil
	}
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, ErrTaskNotPending) {
			return nil
		}
		return fmt.Errorf("failed to mark task as processing: %w", err)
	}
	task.Status = TaskStatusProcessing
	return nil
}

func standardSnapshotFromTask(task *Task) (*StandardProductSnapshot, error) {
	if task == nil || task.Result == nil {
		return nil, fmt.Errorf("standard product snapshot is required before platform adaptation")
	}
	if task.Result.StandardProductSnapshot != nil {
		return task.Result.StandardProductSnapshot, nil
	}
	snapshot := buildStandardProductSnapshot(task.Result)
	if standardProductSnapshotEmpty(snapshot) {
		return nil, fmt.Errorf("standard product snapshot is required before platform adaptation")
	}
	return snapshot, nil
}

func (s *service) loadPlatformAdaptationAssets(ctx context.Context, task *Task, snapshot *StandardProductSnapshot) (*assetpkg.Inventory, []assetgeneration.Task) {
	assetRepo := resolveWorkflowAssetRepository(s)
	var inventory *assetpkg.Inventory
	if assetRepo != nil {
		saved, err := assetRepo.GetInventory(ctx, assetpkg.InventoryRef{TaskID: task.ID})
		if err == nil {
			inventory = saved
		} else if !errors.Is(err, assetrepo.ErrInventoryNotFound) {
			inventory = nil
		}
	}
	if inventory == nil && snapshot != nil {
		inventory = assetpkg.BuildInventory(task.ID, snapshot.AssetBundle)
	}
	var persistedGenerationTasks []assetgeneration.Task
	if assetRepo != nil {
		if savedTasks, err := assetRepo.ListGenerationTasks(ctx, task.ID); err == nil {
			persistedGenerationTasks = cloneGenerationTasks(savedTasks)
		}
	}
	return inventory, persistedGenerationTasks
}

func (s *service) persistProcessedTaskResult(ctx context.Context, taskID string, result *ListingKitResult) error {
	if result == nil {
		return fmt.Errorf("listing kit result is nil")
	}
	if result.Summary != nil && result.Summary.NeedsReview {
		result.Status = string(TaskStatusNeedsReview)
		result.ReviewReasons = reviewReasonsFromResult(result)
		return s.repo.MarkNeedsReview(ctx, taskID, result, taskNeedsReviewReason(result))
	}
	result.Status = string(TaskStatusCompleted)
	return s.repo.MarkCompleted(ctx, taskID, result)
}
