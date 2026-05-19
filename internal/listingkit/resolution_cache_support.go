package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func buildSheinResolutionCacheSummary(pkg *sheinpub.Package) *SheinResolutionCacheSummary {
	if pkg == nil {
		return nil
	}
	summary := &SheinResolutionCacheSummary{}
	if pkg.CategoryResolution != nil {
		summary.Category = sheinpub.CloneResolutionCacheInfo(pkg.CategoryResolution.Cache)
	}
	if pkg.AttributeResolution != nil {
		summary.Attributes = sheinpub.CloneResolutionCacheInfo(pkg.AttributeResolution.Cache)
	}
	if pkg.SaleAttributeResolution != nil {
		summary.SaleAttributes = sheinpub.CloneResolutionCacheInfo(pkg.SaleAttributeResolution.Cache)
	}
	if summary.Category == nil && summary.Attributes == nil && summary.SaleAttributes == nil {
		return nil
	}
	return summary
}
