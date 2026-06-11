package listingkit

import (
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskSubmissionExecutionService) normalizeSheinSubmitPackage(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	normalizeSheinStudioSubmitSupplierSKUs(task, pkg, normalizedSubmitIdempotencyKey(req))
	var manualOverrides map[string]float64
	if pkg.FinalSubmissionDraft != nil {
		manualOverrides = pkg.FinalSubmissionDraft.ManualPriceOverrides
	}
	if pkg.Pricing == nil || !pkg.Pricing.Ready || len(manualOverrides) > 0 {
		review := buildSheinDraftBackedPricingReview(pkg, s.currentSheinPricingRule(), manualOverrides)
		applySheinPricingReview(pkg, review)
	} else {
		applySheinPricingReview(pkg, pkg.Pricing)
	}
	applyConfirmedFinalSubmissionDraft(pkg, req, action)
	repairSheinSubmitSaleAttributes(pkg)
	applySheinFinalImageDraft(pkg)
	applySheinVariantImageCoverageGuard(task.Result, task.Request, pkg)
}

func applyConfirmedFinalSubmissionDraft(pkg *SheinPackage, req *SubmitTaskRequest, action string) {
	if pkg == nil || req == nil || !req.ConfirmedFinal {
		return
	}
	if pkg.FinalSubmissionDraft == nil {
		pkg.FinalSubmissionDraft = &sheinpub.FinalDraft{}
	}
	now := time.Now()
	pkg.FinalSubmissionDraft.Confirmed = true
	pkg.FinalSubmissionDraft.ConfirmedAt = &now
	pkg.FinalSubmissionDraft.UpdatedAt = &now
	if pkg.FinalSubmissionDraft.SubmitMode == "" {
		pkg.FinalSubmissionDraft.SubmitMode = action
	}
}
