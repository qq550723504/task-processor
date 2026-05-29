package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
)

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
	applySheinVariantImageCoverageGuard(final, task.Request, final.Shein)
}
