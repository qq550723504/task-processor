package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

func (s *service) buildRetryGenerationTaskSelection(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
	selected, err := selectGenerationTasksForRetry(existing, task.Result, req)
	if err != nil {
		return nil, err
	}
	planned, err := s.planMissingRetryGenerationTasks(ctx, task, inventory, existing, req)
	if err != nil {
		return nil, err
	}
	if len(planned) == 0 {
		return selected, nil
	}
	return mergeRetrySelection(selected, planned), nil
}

func (s *service) planMissingRetryGenerationTasks(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
	if s == nil || task == nil || task.Result == nil || s.assetGenerator == nil || s.assetRecipeResolver == nil {
		return nil, nil
	}
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, task.Result.CanonicalProduct)
	if len(recipesByPlatform) == 0 {
		return nil, nil
	}
	planned, err := s.assetGenerator.Plan(ctx, assetgeneration.Request{
		TaskID:    task.ID,
		Product:   effectiveCatalogProduct(task.Result),
		Inventory: inventory,
		Recipes:   flattenRecipes(recipesByPlatform),
	})
	if err != nil || planned == nil || len(planned.Tasks) == 0 {
		return nil, err
	}
	queueIndex := retryGenerationQueueIndex(existing, task.Result)
	slotFilters := normalizeRetrySlots(req.Slots)
	existingKeys := make(map[generationQueueKey]struct{}, len(existing))
	for _, item := range existing {
		existingKeys[generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)] = struct{}{}
	}
	out := make([]assetgeneration.Task, 0, len(planned.Tasks))
	for _, item := range planned.Tasks {
		if !generationTaskRetryable(item) {
			continue
		}
		key := generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)
		if _, exists := existingKeys[key]; exists {
			continue
		}
		if _, ok := queueIndex[key]; !ok {
			continue
		}
		if !generationTaskMatchesRetryFilter(item, queueIndex, slotFilters, req) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func retryGenerationQueueIndex(existing []assetgeneration.Task, result *ListingKitResult) map[generationQueueKey]GenerationWorkQueueItem {
	queueResult := &ListingKitResult{AssetGenerationTasks: append([]assetgeneration.Task(nil), existing...)}
	if result != nil {
		queueResult.Amazon = result.Amazon
		queueResult.Shein = result.Shein
		queueResult.Temu = result.Temu
		queueResult.Walmart = result.Walmart
	}
	return indexGenerationWorkQueue(buildGenerationWorkQueue(queueResult))
}

func mergeRetrySelection(selected []assetgeneration.Task, planned []assetgeneration.Task) []assetgeneration.Task {
	if len(planned) == 0 {
		return selected
	}
	out := append([]assetgeneration.Task(nil), selected...)
	seen := make(map[generationQueueKey]struct{}, len(selected)+len(planned))
	for _, item := range selected {
		seen[generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)] = struct{}{}
	}
	for _, item := range planned {
		key := generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}
