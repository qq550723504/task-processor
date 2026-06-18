package listingkit

import (
	"context"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/listingkit/sheinadapter"
	sheinpub "task-processor/internal/publishing/shein"

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

type platformPostprocessPhase struct {
	service *service
}

func buildPlatformPostprocessPhase(s *service) *platformPostprocessPhase {
	return &platformPostprocessPhase{service: s}
}

func (p *platformPostprocessPhase) run(
	ctx context.Context,
	task *Task,
	final *ListingKitResult,
	sdsOptions *SDSSyncOptions,
) {
	if final.Shein != nil {
		if err := sheinpub.OptimizePackageReviewContent(ctx, final.Shein, sheinadapter.NewReviewContentOptimizer(resolveWorkflowSheinContentOptimizer(p.service))); err != nil {
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
}

type platformReviewPhase struct{}

func buildPlatformReviewPhase() *platformReviewPhase {
	return &platformReviewPhase{}
}

func (p *platformReviewPhase) run(
	final *ListingKitResult,
	snapshot *StandardProductSnapshot,
) {
	if final.Summary == nil {
		final.Summary = &GenerationSummary{}
	}
	if snapshot != nil && snapshot.Summary != nil {
		final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, snapshot.Summary.Warnings...))
	}

	sheinReviewStage := newWorkflowRecorder(final).Start("shein_review", "")
	applySheinInspectionReviewToSummary(final)
	applySheinVariantCoverageReviewToSummary(final)
	addSheinReviewWorkflowIssues(final)
	sheinReviewStage.Complete()
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
