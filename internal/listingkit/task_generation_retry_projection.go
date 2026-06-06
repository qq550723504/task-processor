package listingkit

import (
	asset "task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type retryGenerationProjectionPhase struct {
	assetRecipeResolver assetrecipe.Resolver
	assetBundleBuilder  assetbundle.Builder
}

func buildRetryGenerationProjectionPhase(resolver assetrecipe.Resolver, builder assetbundle.Builder) *retryGenerationProjectionPhase {
	return &retryGenerationProjectionPhase{
		assetRecipeResolver: resolver,
		assetBundleBuilder:  builder,
	}
}

func (p *retryGenerationProjectionPhase) run(
	task *Task,
	inventory *asset.Inventory,
	updatedTasks []assetgeneration.Task,
	selectedTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
	reviews []GenerationReviewRecord,
) (*ListingKitResult, *GenerationTaskPage) {
	if task == nil || task.Result == nil {
		return nil, nil
	}

	rebuiltResult := *task.Result
	rebuiltResult.AssetBundle = rebuildBundleFromInventory(task.Result.AssetBundle, inventory)
	if inventory != nil {
		rebuiltResult.AssetInventorySummary = inventory.Summary
	}

	recipesByPlatform := resolveRecipesForPlatforms(p.assetRecipeResolver, previewPlatforms(task), task.Result.CanonicalProduct)
	attachPlatformImageBundles(&rebuiltResult, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: updatedTasks}, p.assetBundleBuilder)
	decorateListingKitResultGeneration(&rebuiltResult, updatedTasks)
	syncAssetRenderPreviews(&rebuiltResult)
	decorateListingKitResultReview(&rebuiltResult, reviews)

	page := buildGenerationTaskPage(task.ID, task.UpdatedAt, updatedTasks, updatedTasks, generationTaskListPage{
		Page:     defaultGenerationTaskPage,
		PageSize: defaultGenerationTaskPageSize,
		Total:    len(updatedTasks),
	})
	if dispatchResult == nil {
		dispatchResult = &assetgeneration.Result{}
	}
	page.MatchedQueue = buildMatchedGenerationQueue(rebuiltResult.AssetGenerationQueue, selectedTasks)
	page.ExecutedQueue = buildMatchedGenerationQueue(rebuiltResult.AssetGenerationQueue, dispatchResult.Tasks)
	return &rebuiltResult, page
}

func buildMatchedGenerationQueue(queue *GenerationWorkQueue, tasks []assetgeneration.Task) *GenerationWorkQueue {
	if queue == nil || len(tasks) == 0 {
		return &GenerationWorkQueue{Summary: buildGenerationWorkQueueSummary(nil)}
	}
	keys := make(map[generationQueueKey]struct{}, len(tasks))
	for _, task := range tasks {
		keys[generationQueueItemKey(task.Platform, task.RecipeID, task.Slot)] = struct{}{}
	}
	items := make([]GenerationWorkQueueItem, 0, len(tasks))
	for _, item := range queue.Items {
		if _, ok := keys[generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)]; !ok {
			continue
		}
		items = append(items, item)
	}
	return &GenerationWorkQueue{
		Summary: buildGenerationWorkQueueSummary(items),
		Items:   items,
	}
}
