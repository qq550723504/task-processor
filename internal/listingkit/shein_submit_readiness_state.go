package listingkit

func sheinCategoryReviewPending(pkg *SheinPackage) bool {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	return pkg.SaleAttributeResolution.RecommendCategoryReview
}
