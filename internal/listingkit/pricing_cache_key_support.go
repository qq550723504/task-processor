package listingkit

import sheinpub "task-processor/internal/publishing/shein"

type sheinPricingSKUFact = sheinpub.PricingSKUFact

func sheinPricingCacheKey(req *sheinpub.BuildRequest, pkg *sheinpub.Package, rule sheinpub.PricingRule) string {
	return sheinpub.PricingCacheKey(req, pkg, rule)
}

func sheinPricingSourceIdentity(pkg *sheinpub.Package) string {
	return sheinpub.PricingSourceIdentity(pkg)
}

func sortedSheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) []string {
	return sheinpub.SortedPricingSKUFacts(pkg, rule)
}

func sortedSheinPricingSKUAliases(pkg *sheinpub.Package) []string {
	return sheinpub.SortedPricingSKUAliases(pkg)
}

func sheinPricingSKUFacts(pkg *sheinpub.Package, rule sheinpub.PricingRule) map[string]sheinPricingSKUFact {
	return sheinpub.PricingSKUFacts(pkg, rule)
}

func sheinPricingProductIdentity(pkg *sheinpub.Package) []string {
	return sheinpub.PricingProductIdentity(pkg)
}

func sheinPricingSKUAlias(value string) string {
	return sheinpub.PricingSKUAlias(value)
}

func sheinPricingDraftSKUKey(sku *sheinpub.SKUDraft) string {
	return sheinpub.PricingDraftSKUKey(sku)
}

func sheinPricingReviewSKUKey(item sheinpub.SKUPriceReview) string {
	return sheinpub.PricingReviewSKUKey(item)
}

func sheinPricingStoreID(req *sheinpub.BuildRequest) string {
	return sheinpub.PricingStoreID(req)
}

func sheinPricingShortKey(key string) string {
	return sheinpub.PricingShortKey(key)
}
