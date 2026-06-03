package listingkit

func applyAssetGenerationFiltersCloneShape(filters *AssetGenerationRecommendedFilters, cloned *AssetGenerationRecommendedFilters) {
	if filters == nil || cloned == nil {
		return
	}
	applyAssetGenerationFiltersPlatformsClone(filters, cloned)
}
