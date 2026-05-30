package listingkit

import (
	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type platformAssetDispatchMutation struct {
	final           *ListingKitResult
	inventory       *asset.Inventory
	generationTasks []assetgeneration.Task
}

func applyPlatformAssetDispatchMutation(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationTasks []assetgeneration.Task,
	dispatchResult *assetgeneration.Result,
	bundleBuilder assetbundle.Builder,
) platformAssetDispatchMutation {
	mutation := platformAssetDispatchMutation{
		final:           final,
		inventory:       inventory,
		generationTasks: generationTasks,
	}
	if dispatchResult == nil {
		return mutation
	}
	if len(dispatchResult.Assets) > 0 {
		inventory.Records = append(inventory.Records, dispatchResult.Assets...)
		inventory.Summary = rebuildInventorySummary(inventory)
		final.AssetBundle = rebuildBundleWithGeneratedAssets(final.AssetBundle, dispatchResult.Assets)
		final.AssetInventorySummary = inventory.Summary
	}
	attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchResult.Tasks}, bundleBuilder)
	mutation.generationTasks = mergeGenerationTasks(generationTasks, dispatchResult.Tasks)
	return mutation
}
