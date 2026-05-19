package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	assetpkg "task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func (s *service) ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error) {
	ctx, task, err := s.loadTaskExecutionContext(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := s.markTaskProcessingIfPending(ctx, task); err != nil {
		return nil, err
	}
	state, err := s.runStandardProductWorkflow(ctx, task)
	if err != nil {
		if state != nil && state.result != nil {
			state.result.Status = string(TaskStatusProcessing)
			_ = s.repo.SaveTaskResult(ctx, task.ID, state.result)
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}
	state.result.Status = string(TaskStatusProcessing)
	if err := s.repo.SaveTaskResult(ctx, task.ID, state.result); err != nil {
		return nil, err
	}
	if s.platformAdaptWorkflowEnabled && s.platformAdaptWorkflowClient != nil {
		if err := s.platformAdaptWorkflowClient.StartPlatformAdaptation(ctx, PlatformAdaptWorkflowStartInput{
			TaskID:      strings.TrimSpace(task.ID),
			Platform:    "all",
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			return nil, err
		}
	}
	return state.snapshot, nil
}

func (s *service) ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error) {
	ctx, task, err := s.loadTaskExecutionContext(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := s.markTaskProcessingIfPending(ctx, task); err != nil {
		return nil, err
	}
	snapshot, err := standardSnapshotFromTask(task)
	if err != nil {
		return nil, err
	}
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, snapshot.CanonicalProduct)
	if normalized := strings.ToLower(strings.TrimSpace(platform)); normalized != "" && normalized != "all" {
		filtered := map[string][]assetrecipe.AssetRecipe{}
		if recipes, ok := recipesByPlatform[normalized]; ok {
			filtered[normalized] = recipes
		} else {
			filtered[normalized] = nil
		}
		recipesByPlatform = filtered
	}
	inventory, persistedGenerationTasks := s.loadPlatformAdaptationAssets(ctx, task, snapshot)
	var generationPlan *assetgeneration.Result
	if len(persistedGenerationTasks) > 0 {
		generationPlan = &assetgeneration.Result{Tasks: cloneGenerationTasks(persistedGenerationTasks)}
	}
	var sdsOptions *SDSSyncOptions
	if task.Request != nil && task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	result := s.runPlatformAdaptation(
		ctx,
		task,
		snapshot,
		recipesByPlatform,
		generationPlan,
		inventory,
		persistedGenerationTasks,
		shouldGenerateAssets(task.Request),
		sdsOptions,
	)
	if err := s.persistProcessedTaskResult(ctx, task.ID, result); err != nil {
		return nil, err
	}
	return result, nil
}

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
	ctx = openaiclient.WithIdentity(ctx, openaiclient.Identity{
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
	var inventory *assetpkg.Inventory
	if s.assetRepo != nil {
		saved, err := s.assetRepo.GetInventory(ctx, assetpkg.InventoryRef{TaskID: task.ID})
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
	if s.assetRepo != nil {
		if savedTasks, err := s.assetRepo.ListGenerationTasks(ctx, task.ID); err == nil {
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
