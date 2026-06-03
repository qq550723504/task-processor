package listingkit

func applyAssetGenerationFiltersPlatformsClone(filters *AssetGenerationRecommendedFilters, cloned *AssetGenerationRecommendedFilters) {
	if filters == nil || cloned == nil {
		return
	}
	cloned.Platforms = append([]string(nil), filters.Platforms...)
}
