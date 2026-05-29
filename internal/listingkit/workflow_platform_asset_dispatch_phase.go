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
			if len(dispatchResult.Assets) > 0 {
				inventory.Records = append(inventory.Records, dispatchResult.Assets...)
				inventory.Summary = rebuildInventorySummary(inventory)
				final.AssetBundle = rebuildBundleWithGeneratedAssets(final.AssetBundle, dispatchResult.Assets)
				final.AssetInventorySummary = inventory.Summary
				if p.service.assetRepo != nil {
					_ = p.service.assetRepo.SaveInventory(ctx, inventory)
				}
			}
			attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchResult.Tasks}, p.service.assetBundleBuilder)
			persistedGenerationTasks = mergeGenerationTasks(persistedGenerationTasks, dispatchResult.Tasks)
		}
		if dispatchErr == nil {
			deferredStage.Complete()
		}
	}
	decorateListingKitResultGeneration(final, persistedGenerationTasks)
	if p.service.assetRepo != nil && len(persistedGenerationTasks) > 0 {
		if err := p.service.assetRepo.SaveGenerationTasks(ctx, task.ID, persistedGenerationTasks); err != nil {
			appendWarning(final, "asset generation task persistence failed: "+err.Error())
			newWorkflowRecorder(final).AddIssue(WorkflowIssueSeverityWarning, "asset_generation_platform", "asset_generation_task_persistence_failed", "Asset generation task persistence failed", err.Error())
		}
	}
	return persistedGenerationTasks
}
