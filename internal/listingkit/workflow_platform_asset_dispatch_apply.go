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
		_ = inventory.Records
		_ = final.AssetBundle
		buildPlatformAssetDispatchInventoryApplyPhase().run(final, inventory, dispatchResult.Assets)
	}
	mutation.generationTasks = buildPlatformAssetDispatchBundleApplyPhase(bundleBuilder).run(
		final,
		inventory,
		recipesByPlatform,
		generationTasks,
		dispatchResult.Tasks,
	)
	return mutation
}
