package listingkit

func applyAssetGenerationRetryOrientedFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	if applyAssetGenerationFailedRetryFilterMutation(actionKey, filters) {
		return true
	}
	return applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters)
}
