package listingkit

func applyAssetGenerationRegularActionKeyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	if applyAssetGenerationRetryOrientedFilterMutation(actionKey, filters) {
		return
	}
	if applyAssetGenerationReviewReadyFilterMutation(actionKey, filters) {
		return
	}
	applyAssetGenerationMissingSlotFilterMutation(actionKey, filters)
}
