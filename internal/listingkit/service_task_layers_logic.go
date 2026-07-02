package listingkit

import (
	"context"
	"strings"
	"time"

	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
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
			_ = s.repo.SaveTaskResult(ctx, task.ID, mergeStandardProductLayerResult(task.Result, state.result))
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}
	state.result.Status = string(TaskStatusProcessing)
	if err := s.repo.SaveTaskResult(ctx, task.ID, mergeStandardProductLayerResult(task.Result, state.result)); err != nil {
		return nil, err
	}
	if client, enabled := resolvePlatformAdaptWorkflowClient(s); enabled && client != nil {
		if err := client.StartPlatformAdaptation(ctx, PlatformAdaptWorkflowStartInput{
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
	assetRecipeResolver := resolveWorkflowAssetRecipeResolver(s)
	recipesByPlatform := resolveRecipesForPlatforms(assetRecipeResolver, task.Request.Platforms, snapshot.CanonicalProduct)
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
		generationPlan = &assetgeneration.Result{Tasks: assetgeneration.CloneTasks(persistedGenerationTasks)}
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
