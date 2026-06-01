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
