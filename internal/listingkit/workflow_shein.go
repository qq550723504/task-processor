package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func (s *service) applyDefaultSheinPricing(pkg *sheinpub.Package) {
	if pkg == nil || (pkg.Pricing != nil && pkg.Pricing.Ready) {
		return
	}
	var overrides map[string]float64
	if pkg.FinalDraft != nil {
		overrides = pkg.FinalDraft.ManualPriceOverrides
	}
	review := buildSheinPricingReview(pkg, s.currentSheinPricingRule(), overrides)
	applySheinPricingReview(pkg, review)
}
