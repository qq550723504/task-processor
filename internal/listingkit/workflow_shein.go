package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func (s *service) applyDefaultSheinPricing(req *GenerateRequest, pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || (pkg.Pricing != nil && pkg.Pricing.Ready) {
		return
	}
	if cached := s.loadSheinPricingCache(req, pkg); cached != nil {
		applySheinPricingReview(pkg, cached)
		return
	}
	var overrides map[string]float64
	if pkg.FinalSubmissionDraft != nil {
		overrides = pkg.FinalSubmissionDraft.ManualPriceOverrides
	}
	review := buildSheinDraftBackedPricingReview(pkg, s.currentSheinPricingRule(), overrides)
	applySheinPricingReview(pkg, review)
	logPricingCacheEvent("miss", buildSheinPublishRequest(req), pkg, review.Cache, nil)
}
