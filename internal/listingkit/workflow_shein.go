package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func (s *service) applyDefaultSheinPricing(req *GenerateRequest, pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if reason := sheinPricingCacheSkipReason(pkg); reason != "" {
		logPricingCacheEvent("skip", buildSheinPublishRequest(req), pkg, sheinPricingCacheInfo(pkg), map[string]any{
			"reason": reason,
		})
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

func sheinPricingCacheSkipReason(pkg *sheinpub.Package) string {
	switch {
	case pkg == nil:
		return "package_nil"
	case pkg.Pricing != nil && pkg.Pricing.Ready:
		return "existing_ready_pricing"
	default:
		return ""
	}
}

func sheinPricingCacheInfo(pkg *sheinpub.Package) *sheinpub.ResolutionCacheInfo {
	if pkg == nil || pkg.Pricing == nil {
		return nil
	}
	return pkg.Pricing.Cache
}
