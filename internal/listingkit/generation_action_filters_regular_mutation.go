package listingkit

func applyAssetGenerationRegularActionKeyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	if applyAssetGenerationFailedRetryFilterMutation(actionKey, filters) {
		return
	}
	if applyAssetGenerationProvisionalRetryFilterMutation(actionKey, filters) {
		return
	}
	if applyAssetGenerationReviewReadyFilterMutation(actionKey, filters) {
		return
	}
	applyAssetGenerationMissingSlotFilterMutation(actionKey, filters)
}
