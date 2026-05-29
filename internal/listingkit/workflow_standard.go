package listingkit

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
	"task-processor/internal/productimage"
)

type standardWorkflowState struct {
	result                   *ListingKitResult
	snapshot                 *StandardProductSnapshot
	recipesByPlatform        map[string][]assetrecipe.AssetRecipe
	generationPlan           *assetgeneration.Result
	inventory                *asset.Inventory
	persistedGenerationTasks []assetgeneration.Task
	enableAssetGeneration    bool
	sdsOptions               *SDSSyncOptions
}

func (s *service) runStandardProductWorkflow(ctx context.Context, task *Task) (*standardWorkflowState, error) {
	result := initResult(task)
	recorder := newWorkflowRecorder(result)
	enableAssetGeneration := shouldGenerateAssets(task.Request)
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/workflow_standard",
		"task_id":   task.ID,
	})

	canonicalProduct, err := buildStandardWorkflowCanonicalPhase(s).run(ctx, task, result, recorder, log)
	if err != nil {
		return &standardWorkflowState{result: result}, err
	}

	result.CanonicalProduct = canonicalProduct
	result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
	result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
	result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
	log.WithFields(logrus.Fields{
		"has_canonical": canonicalProduct != nil,
		"image_count": func() int {
			if canonicalProduct == nil {
				return 0
			}
			return len(canonicalProduct.Images)
		}(),
		"variant_count": func() int {
			if canonicalProduct == nil {
				return 0
			}
			return len(canonicalProduct.Variants)
		}(),
	}).Info("canonical product prepared for listing kit workflow")
	if persistErr := s.persistSDSBaselineIfEligible(ctx, task); persistErr != nil {
		log.WithError(persistErr).Warn("sds baseline persistence failed")
	} else if validationErr := s.persistSDSBaselineValidation(ctx, task); validationErr != nil {
		log.WithError(validationErr).Warn("sds baseline validation persistence failed")
	}

	var imageResult *productimage.ImageProcessResult
	if shouldProcessImages(task.Request) && s.imageSvc != nil {
		stage := recorder.Start("product_image", "")
		imageTask, imageErr := s.imageSvc.CreateProcessTask(productimage.WithInlineTaskExecution(ctx), toImageProcessRequest(task))
		if imageErr != nil {
			markChildTask(result, "product_image", "", string(TaskStatusFailed), imageErr.Error())
			appendWarning(result, "image processing skipped: "+imageErr.Error())
			stage.Degrade("image_processing_skipped", "Image processing skipped", imageErr.Error())
		} else {
			stage.SetTaskID(imageTask.ID)
			markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusPending), "")
			imageResult, imageErr = s.imageSvc.ProcessImages(ctx, imageTask)
			if imageErr != nil {
				markChildTask(result, "product_image", imageTask.ID, string(TaskStatusFailed), imageErr.Error())
				appendWarning(result, "image processing failed: "+imageErr.Error())
				stage.Degrade("image_processing_failed", "Image processing failed", imageErr.Error())
			} else {
				markChildTask(result, "product_image", imageTask.ID, string(productimage.TaskStatusCompleted), "")
				stage.Complete()
				result.ImageAssets = imageResult
				result.AssetBundle = asset.BuildBundle(canonicalProduct, imageResult)
				result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
				s.syncSDSDesign(ctx, task, result, imageResult, recorder)
			}
		}
	}
	if imageResult == nil && shouldRunRemoteSDSDesignSync(task.Request) {
		log.Info("starting remote SDS design sync for listing kit workflow")
		s.syncSDSDesignFromRemote(ctx, task, result, recorder)
		log.WithFields(logrus.Fields{
			"sds_status": func() string {
				if result.SDSDesignResult == nil {
					return ""
				}
				return result.SDSDesignResult.Status
			}(),
			"sds_error": func() string {
				if result.SDSDesignResult == nil {
					return ""
				}
				return result.SDSDesignResult.Error
			}(),
		}).Info("finished remote SDS design sync for listing kit workflow")
	}
	var sdsOptions *SDSSyncOptions
	if task.Request.Options != nil {
		sdsOptions = task.Request.Options.SDS
	}
	if applySDSSyncMetadataToCanonical(canonicalProduct, result.SDSDesignResult, sdsOptions) {
		result.CatalogProduct = catalog.BuildProduct(canonicalProduct)
		result.AssetBundle = asset.BuildBundle(canonicalProduct, result.ImageAssets)
		result.AssetInventorySummary = buildInventorySummaryFromBundle(result.AssetBundle)
		log.Info("applied SDS sync metadata to canonical product")
	}

	inventory := asset.BuildInventory(task.ID, result.AssetBundle)
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, canonicalProduct)
	baseRecipes := baselineGenerationRecipes()
	var generationPlan *assetgeneration.Result
	var persistedGenerationTasks []assetgeneration.Task
	if inventory != nil {
		inventoryStage := recorder.Start("asset_inventory", "")
		if s.assetRepo != nil {
			if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {
				appendWarning(result, "asset inventory persistence failed: "+err.Error())
				inventoryStage.Degrade("asset_inventory_persistence_failed", "Asset inventory persistence failed", err.Error())
			} else {
				inventoryStage.Complete()
			}
		} else {
			inventoryStage.Skip()
		}
		if enableAssetGeneration && s.assetGenerator != nil && len(baseRecipes) > 0 {
			stage := recorder.Start("asset_generation_baseline", "")
			execution, execErr := s.assetGenerator.Execute(ctx, assetgeneration.Request{
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
				inventory.Summary = rebuildInventorySummary(inventory)
				result.AssetBundle = rebuildBundleWithGeneratedAssets(result.AssetBundle, execution.Assets)
				if s.assetRepo != nil {
					_ = s.assetRepo.SaveInventory(ctx, inventory)
				}
			}
		}
		if enableAssetGeneration && s.assetGenerator != nil && s.assetRecipeResolver != nil {
			stage := recorder.Start("asset_generation_platform", "")
			var planErr error
			generationPlan, planErr = s.assetGenerator.Plan(ctx, assetgeneration.Request{
				TaskID:    task.ID,
				Product:   result.CatalogProduct,
				Inventory: inventory,
				Recipes:   flattenRecipes(recipesByPlatform),
			})
			if planErr != nil {
				stage.Degrade("asset_generation_platform_plan_failed", "Platform asset generation planning failed", planErr.Error())
			}
			if generationPlan != nil && len(generationPlan.Tasks) > 0 {
				dispatchResult, dispatchErr := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
					TaskID:    task.ID,
					Product:   result.CatalogProduct,
					Inventory: inventory,
					Tasks:     generationPlan.Tasks,
				})
				if dispatchErr != nil {
					stage.Degrade("asset_generation_platform_dispatch_failed", "Platform asset generation dispatch failed", dispatchErr.Error())
				}
				if dispatchResult != nil {
					generationPlan.Tasks = cloneGenerationTasks(dispatchResult.Tasks)
					persistedGenerationTasks = mergeGenerationTasks(persistedGenerationTasks, dispatchResult.Tasks)
					if len(dispatchResult.Assets) > 0 {
						inventory.Records = append(inventory.Records, dispatchResult.Assets...)
						inventory.Summary = rebuildInventorySummary(inventory)
						result.AssetBundle = rebuildBundleWithGeneratedAssets(result.AssetBundle, dispatchResult.Assets)
						if s.assetRepo != nil {
							_ = s.assetRepo.SaveInventory(ctx, inventory)
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

	snapshot := buildStandardProductSnapshot(result)
	result.StandardProductSnapshot = snapshot
	return &standardWorkflowState{
		result:                   result,
		snapshot:                 snapshot,
		recipesByPlatform:        recipesByPlatform,
		generationPlan:           generationPlan,
		inventory:                inventory,
		persistedGenerationTasks: persistedGenerationTasks,
		enableAssetGeneration:    enableAssetGeneration,
		sdsOptions:               sdsOptions,
	}, nil
}
