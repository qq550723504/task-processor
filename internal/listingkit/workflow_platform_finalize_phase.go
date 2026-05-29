package listingkit

import (
	"context"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	sheinpub "task-processor/internal/publishing/shein"
)

type platformFinalizePhase struct {
	service *service
}

func buildPlatformFinalizePhase(s *service) *platformFinalizePhase {
	return &platformFinalizePhase{service: s}
}

func (p *platformFinalizePhase) run(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	snapshot *StandardProductSnapshot,
	recipesByPlatform map[string][]assetrecipe.AssetRecipe,
	generationPlan *assetgeneration.Result,
	inventory *asset.Inventory,
	persistedGenerationTasks []assetgeneration.Task,
	enableAssetGeneration bool,
	sdsOptions *SDSSyncOptions,
) *ListingKitResult {
	if final.Shein != nil {
		if err := sheinpub.OptimizePackageReviewContent(ctx, final.Shein, p.service.sheinContentOptimizer); err != nil {
			appendWarning(final, "shein content optimization skipped: "+err.Error())
		}
	}
	p.service.applyDefaultSheinPricing(task.Request, final.Shein)
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
	}

	newWorkflowRecorder(final).FinalizeSummary()
	syncAssetRenderPreviews(final)
	logrus.WithFields(logrus.Fields{
		"component":     "listingkit/platform_adaptation_finalize",
		"task_id":       task.ID,
		"needs_review":  final.Summary != nil && final.Summary.NeedsReview,
		"warning_count": processWarningCount(final),
	}).Info("listing kit platform adaptation finalized")
	return final
}
