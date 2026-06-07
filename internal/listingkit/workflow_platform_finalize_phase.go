package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"

	"github.com/sirupsen/logrus"
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
	buildPlatformPostprocessPhase(p.service).run(ctx, task, final, sdsOptions)
	buildPlatformReviewPhase().run(final, snapshot)
	applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)
	persistedGenerationTasks = buildPlatformAssetDispatchPhase(p.service).run(
		ctx,
		task,
		final,
		inventory,
		recipesByPlatform,
		generationPlan,
		persistedGenerationTasks,
		enableAssetGeneration,
	)
	return buildPlatformSummaryPhase().run(task, final)
}

type platformSummaryPhase struct{}

func buildPlatformSummaryPhase() *platformSummaryPhase {
	return &platformSummaryPhase{}
}

func (p *platformSummaryPhase) run(task *Task, final *ListingKitResult) *ListingKitResult {
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
