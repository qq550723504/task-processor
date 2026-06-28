package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog/canonical"
)

type standardWorkflowAssetPhase struct {
	service *service
}

func buildStandardWorkflowAssetPhase(s *service) *standardWorkflowAssetPhase {
	return &standardWorkflowAssetPhase{service: s}
}

func (p *standardWorkflowAssetPhase) run(
	ctx context.Context,
	task *Task,
	result *ListingKitResult,
	canonicalProduct *canonical.Product,
	recorder *workflowRecorder,
	enableAssetGeneration bool,
) (*asset.Inventory, map[string][]assetrecipe.AssetRecipe, *assetgeneration.Result, []assetgeneration.Task) {
	inventory := asset.BuildInventory(task.ID, result.AssetBundle)
	assetRepo := resolveWorkflowAssetRepository(p.service)
	assetGenerator := resolveWorkflowAssetGenerationService(p.service)
	assetRecipeResolver := resolveWorkflowAssetRecipeResolver(p.service)
	recipesByPlatform := resolveRecipesForPlatforms(assetRecipeResolver, task.Request.Platforms, canonicalProduct)
	baseRecipes := baselineGenerationRecipes()
	var generationPlan *assetgeneration.Result
	var persistedGenerationTasks []assetgeneration.Task

	if inventory != nil {
		inventoryStage := recorder.Start("asset_inventory", "")
		if assetRepo != nil {
			if err := assetRepo.SaveInventory(ctx, inventory); err != nil {
				appendWarning(result, "asset inventory persistence failed: "+err.Error())
				inventoryStage.Degrade("asset_inventory_persistence_failed", "Asset inventory persistence failed", err.Error())
			} else {
				inventoryStage.Complete()
			}
		} else {
			inventoryStage.Skip()
		}
		if enableAssetGeneration && assetGenerator != nil && len(baseRecipes) > 0 {
			stage := recorder.Start("asset_generation_baseline", "")
			execution, execErr := assetGenerator.Execute(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   append([]assetrecipe.AssetRecipe(nil), baseRecipes...),
			})
			if execErr != nil {
				stage.Degrade("asset_generation_baseline_execute_failed", "Baseline asset generation failed", execErr.Error())
			} else {
				stage.Complete()
			}
			if execution != nil && len(execution.Assets) > 0 {
				inventory.Records = append(inventory.Records, execution.Assets...)
				inventory.Summary = asset.RebuildInventorySummary(inventory)
				result.AssetBundle = asset.RebuildBundleWithRecords(result.AssetBundle, execution.Assets)
				if assetRepo != nil {
					_ = assetRepo.SaveInventory(ctx, inventory)
				}
			}
		}
		if enableAssetGeneration && assetGenerator != nil && assetRecipeResolver != nil {
			stage := recorder.Start("asset_generation_platform", "")
			var planErr error
			generationPlan, planErr = assetGenerator.Plan(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   flattenRecipes(recipesByPlatform),
			})
			if planErr != nil {
				stage.Degrade("asset_generation_platform_plan_failed", "Platform asset generation planning failed", planErr.Error())
			}
			if generationPlan != nil && len(generationPlan.Tasks) > 0 {
				dispatchResult, dispatchErr := assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
					TaskID:    task.ID,
					Product:   result.CatalogProduct,
					Inventory: inventory,
					Tasks:     generationPlan.Tasks,
				})
				if dispatchErr != nil {
					stage.Degrade("asset_generation_platform_dispatch_failed", "Platform asset generation dispatch failed", dispatchErr.Error())
				}
				if dispatchResult != nil {
					generationPlan.Tasks = assetgeneration.CloneTasks(dispatchResult.Tasks)
					persistedGenerationTasks = assetgeneration.MergeTasks(persistedGenerationTasks, dispatchResult.Tasks)
					if len(dispatchResult.Assets) > 0 {
						inventory.Records = append(inventory.Records, dispatchResult.Assets...)
						inventory.Summary = asset.RebuildInventorySummary(inventory)
						result.AssetBundle = asset.RebuildBundleWithRecords(result.AssetBundle, dispatchResult.Assets)
						if assetRepo != nil {
							_ = assetRepo.SaveInventory(ctx, inventory)
						}
					}
				}
			}
			if stage.IsRunning() {
				stage.Complete()
			}
		}
		result.AssetInventorySummary = inventory.Summary
		if result.AssetInventorySummary != nil {
			result.AssetInventorySummary.RecipeCount = len(baseRecipes) + len(flattenRecipes(recipesByPlatform))
		}
	}

	return inventory, recipesByPlatform, generationPlan, persistedGenerationTasks
}
