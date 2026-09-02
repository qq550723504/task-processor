package listingkit

import (
	"context"
	"time"

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
) *ListingKitResult {
	buildPlatformPostprocessPhase(p.service).run(ctx, task, final, nil)
	buildPlatformReviewPhase().run(final, snapshot)
	applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)
	return buildPlatformSummaryPhase().run(task, final)
}

type platformPostprocessPhase struct {
	service *service
}

const sheinReviewContentOptimizationTimeout = time.Minute

func withSheinReviewContentOptimizationTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, sheinReviewContentOptimizationTimeout)
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
		optimizationCtx, cancel := withSheinReviewContentOptimizationTimeout(ctx)
		err := sheinpub.OptimizePackageReviewContent(optimizationCtx, final.Shein, sheinadapter.NewReviewContentOptimizer(resolveWorkflowSheinContentOptimizer(p.service)))
		cancel()
		if err != nil {
			appendWarning(final, "shein content optimization skipped: "+err.Error())
		}
	}
	p.service.applyDefaultSheinSizeAttributes(task.Request, final.Shein)
	p.service.applyDefaultSheinPricing(task.Request, final.Shein)
	if shouldSyncSDS(task.Request) {
		applySDSOfficialImagesToShein(final.Shein, task.Request, final.SDSDesignResult, sdsOptions)
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
	logrus.WithFields(logrus.Fields{
		"component":     "listingkit/platform_adaptation_finalize",
		"task_id":       task.ID,
		"needs_review":  final.Summary != nil && final.Summary.NeedsReview,
		"warning_count": processWarningCount(final),
	}).Info("listing kit platform adaptation finalized")
	return final
}
