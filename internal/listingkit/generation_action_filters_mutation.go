package listingkit

func applyAssetGenerationActionFiltersMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	if filters == nil {
		return
	}
	if applyAssetGenerationPreviewCapabilityFilterMutation(actionKey, filters) {
		return
	}
	applyAssetGenerationRegularActionKeyFilterMutation(actionKey, filters)
}
