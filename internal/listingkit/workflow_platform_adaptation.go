package listingkit

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) runPlatformAdaptation(
	ctx context.Context,
	task *Task,
	snapshot *StandardProductSnapshot,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationPlan *assetgeneration.Result,
	inventory *asset.Inventory,
	persistedGenerationTasks []assetgeneration.Task,
	enableAssetGeneration bool,
	sdsOptions *SDSSyncOptions,
) *ListingKitResult {
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/platform_adaptation",
		"task_id":   task.ID,
	})

	var canonicalProduct *canonical.Product
	var imageAssets *productimage.ImageProcessResult
	if snapshot != nil {
		canonicalProduct = snapshot.CanonicalProduct
		imageAssets = snapshot.ImageAssets
	}

	log.Info("starting listing kit platform adaptation")
	final := s.assembler.Assemble(task, canonicalProduct, imageAssets)
	if final == nil {
		final = initResult(task)
	}
	applyStandardProductSnapshot(final, snapshot)

	if final.Shein != nil {
		if err := sheinpub.OptimizePackageReviewContent(ctx, final.Shein, s.sheinContentOptimizer); err != nil {
			appendWarning(final, "shein content optimization skipped: "+err.Error())
		}
	}
	s.applyDefaultSheinPricing(task.Request, final.Shein)
	if shouldUseSDSOfficialImages(task.Request) {
		applySDSOfficialImagesToShein(final.Shein, task.Request, final.SDSDesignResult, sdsOptions)
		applySheinSizeReferenceImages(final.Shein, resolveSheinSizeReferenceImages(task.Request, final.SDSDesignResult))
	}
	if shouldUseSheinStudioAIImages(task.Request) {
		applySheinStudioAIImagesToShein(final.Shein, task.Request, final.SDSDesignResult)
	}
	if final.Summary == nil {
		final.Summary = &GenerationSummary{}
	}
	if snapshot != nil && snapshot.Summary != nil {
		final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, snapshot.Summary.Warnings...))
	}

	sheinReviewStage := newWorkflowRecorder(final).Start("shein_review", "")
	applySheinInspectionReviewToSummary(final)
	addSheinReviewWorkflowIssues(final)
	sheinReviewStage.Complete()
	applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)
	if inventory != nil {
		if enableAssetGeneration {
			attachPlatformImageBundles(final, inventory, recipesByPlatform, generationPlan, s.assetBundleBuilder)
		}
		pendingTasks := collectPlatformGenerationTasks(final)
		if enableAssetGeneration && s.assetGenerator != nil && len(pendingTasks) > 0 {
			deferredStage := newWorkflowRecorder(final).Start("asset_generation_platform", "")
			dispatchResult, dispatchErr := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
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
					if s.assetRepo != nil {
						_ = s.assetRepo.SaveInventory(ctx, inventory)
					}
				}
				attachPlatformImageBundles(final, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: dispatchResult.Tasks}, s.assetBundleBuilder)
				persistedGenerationTasks = mergeGenerationTasks(persistedGenerationTasks, dispatchResult.Tasks)
			}
			if dispatchErr == nil {
				deferredStage.Complete()
			}
		}
		decorateListingKitResultGeneration(final, persistedGenerationTasks)
		if s.assetRepo != nil && len(persistedGenerationTasks) > 0 {
			if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, persistedGenerationTasks); err != nil {
				appendWarning(final, "asset generation task persistence failed: "+err.Error())
				newWorkflowRecorder(final).AddIssue(WorkflowIssueSeverityWarning, "asset_generation_platform", "asset_generation_task_persistence_failed", "Asset generation task persistence failed", err.Error())
			}
		}
	}
	newWorkflowRecorder(final).FinalizeSummary()
	syncAssetRenderPreviews(final)
	log.WithFields(logrus.Fields{
		"needs_review": final.Summary != nil && final.Summary.NeedsReview,
		"warning_count": func() int {
			if final.Summary == nil {
				return 0
			}
			return final.Summary.WarningCount
		}(),
	}).Info("listing kit platform adaptation finished")
	return final
}
