package listingkit

import (
	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type platformAssetDispatchBundleApplyPhase struct {
	bundleBuilder assetbundle.Builder
}

func buildPlatformAssetDispatchBundleApplyPhase(builder assetbundle.Builder) *platformAssetDispatchBundleApplyPhase {
	return &platformAssetDispatchBundleApplyPhase{bundleBuilder: builder}
}

func (p *platformAssetDispatchBundleApplyPhase) run(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationTasks []assetgeneration.Task,
	dispatchTasks []assetgeneration.Task,
) []assetgeneration.Task {
	if p == nil {
		return generationTasks
	}

	buildPlatformAssetDispatchBundleReshapePhase(p.bundleBuilder).run(
		final,
		inventory,
		recipesByPlatform,
		dispatchTasks,
	)

	return buildPlatformAssetDispatchTaskMergePhase().run(generationTasks, dispatchTasks)
}
