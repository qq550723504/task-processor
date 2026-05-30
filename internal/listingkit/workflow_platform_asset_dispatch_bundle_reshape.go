package listingkit

import (
	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type platformAssetDispatchBundleReshapePhase struct {
	bundleBuilder assetbundle.Builder
}

func buildPlatformAssetDispatchBundleReshapePhase(builder assetbundle.Builder) *platformAssetDispatchBundleReshapePhase {
	return &platformAssetDispatchBundleReshapePhase{bundleBuilder: builder}
}

func (p *platformAssetDispatchBundleReshapePhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	dispatchTasks []assetgeneration.Task,
) {
	if p == nil {
		return
	}

	attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchTasks}, p.bundleBuilder)
}
