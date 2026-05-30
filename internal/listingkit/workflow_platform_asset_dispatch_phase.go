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
	p.preAttachBundles(final, inventory, recipesByPlatform, generationPlan, enableAssetGeneration)
	pendingTasks := collectPlatformGenerationTasks(final)
	final, inventory, persistedGenerationTasks = p.dispatchAndApply(
		ctx,
		task,
		final,
		inventory,
		recipesByPlatform,
		persistedGenerationTasks,
		pendingTasks,
		enableAssetGeneration,
	)
	return p.persistHandoff(ctx, task, final, persistedGenerationTasks)
}

func (p *platformAssetDispatchPhase) preAttachBundles(
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationPlan *assetgeneration.Result,
	enableAssetGeneration bool,
) {
	if !enableAssetGeneration {
		return
	}
	attachPlatformImageBundles(final, inventory, recipesByPlatform, generationPlan, p.service.assetBundleBuilder)
}

func (p *platformAssetDispatchPhase) dispatchAndApply(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	inventory *asset.Inventory,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	persistedGenerationTasks []assetgeneration.Task,
	pendingTasks []assetgeneration.Task,
	enableAssetGeneration bool,
) (*ListingKitResult, *asset.Inventory, []assetgeneration.Task) {
	if !enableAssetGeneration || p.service.assetGenerator == nil || len(pendingTasks) == 0 {
		return final, inventory, persistedGenerationTasks
	}
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
	return final, inventory, persistedGenerationTasks
}

func (p *platformAssetDispatchPhase) persistHandoff(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	persistedGenerationTasks []assetgeneration.Task,
) []assetgeneration.Task {
	return buildPlatformAssetDispatchPersistPhase(p.service).run(ctx, task, final, persistedGenerationTasks)
}
