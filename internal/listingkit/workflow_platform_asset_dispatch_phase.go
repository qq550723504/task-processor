package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

type platformAssetDispatchPhase struct {
	service *service
}

func buildPlatformAssetDispatchPhase(s *service) *platformAssetDispatchPhase {
	return &platformAssetDispatchPhase{service: s}
}

func (p *platformAssetDispatchPhase) run(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationPlan *assetgeneration.Result,
	persistedGenerationTasks []assetgeneration.Task,
	enableAssetGeneration bool,
) []assetgeneration.Task {
	if inventory == nil {
		return persistedGenerationTasks
	}
	if enableAssetGeneration {
		attachPlatformImageBundles(final, inventory, recipesByPlatform, generationPlan, p.service.assetBundleBuilder)
	}
	pendingTasks := collectPlatformGenerationTasks(final)
	if enableAssetGeneration && p.service.assetGenerator != nil && len(pendingTasks) > 0 {
		deferredStage := newWorkflowRecorder(final).Start("asset_generation_platform", "")
		dispatchResult, dispatchErr := p.service.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
			TaskID:    task.ID,
			Product:   final.CatalogProduct,
			Inventory: inventory,
			Tasks:     pendingTasks,
		})
		if dispatchErr != nil {
			deferredStage.Degrade("asset_generation_platform_deferred_dispatch_failed", "Deferred platform asset generation dispatch failed", dispatchErr.Error())
		}
		if dispatchResult != nil {
			mutation := applyPlatformAssetDispatchMutation(
				final,
				inventory,
				recipesByPlatform,
				persistedGenerationTasks,
				dispatchResult,
				p.service.assetBundleBuilder,
			)
			final = mutation.final
			inventory = mutation.inventory
			persistedGenerationTasks = mutation.generationTasks
			if len(dispatchResult.Assets) > 0 && p.service.assetRepo != nil {
				_ = p.service.assetRepo.SaveInventory(ctx, inventory)
			}
		}
		if dispatchErr == nil {
			deferredStage.Complete()
		}
	}
	return buildPlatformAssetDispatchPersistPhase(p.service).run(ctx, task, final, persistedGenerationTasks)
}
