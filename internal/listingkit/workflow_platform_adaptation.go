package listingkit

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
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

	if shouldSkipPlatformAdaptationAfterBlockedRemoteSDS(task, snapshot) {
		log.WithField("pod_status", func() string {
			if snapshot == nil || snapshot.PodExecution == nil {
				return ""
			}
			return snapshot.PodExecution.Status
		}()).Warn("skipping platform adaptation because required remote SDS render failed")
		final := initResult(task)
		applyStandardProductSnapshot(final, snapshot)
		final = normalizeListingKitResultSemanticFields(final)
		final.Summary = ensureGenerationSummary(final.Summary)
		if final.PodExecution != nil && strings.TrimSpace(final.PodExecution.FailureReason) != "" {
			reason := strings.TrimSpace(final.PodExecution.FailureReason)
			final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, reason))
			final.ReviewReasons = uniqueStrings(append(final.ReviewReasons, reason))
		}
		final.Summary.NeedsReview = true
		newWorkflowRecorder(final).FinalizeSummary()
		return final
	}

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
	final = buildPlatformFinalizePhase(s).run(
		ctx,
		task,
		final,
		snapshot,
		recipesByPlatform,
		generationPlan,
		inventory,
		persistedGenerationTasks,
		enableAssetGeneration,
		sdsOptions,
	)
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

func shouldSkipPlatformAdaptationAfterBlockedRemoteSDS(task *Task, snapshot *StandardProductSnapshot) bool {
	if task == nil || !shouldRunRemoteSDSDesignSync(task.Request) || snapshot == nil {
		return false
	}
	return podSubmissionBlocked(snapshot.PodExecution)
}
