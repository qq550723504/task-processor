package listingkit

func applyAssetGenerationRegularActionKeyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	if applyAssetGenerationRetryOrientedFilterMutation(actionKey, filters) {
		return
	}
	if applyAssetGenerationReviewReadyFilterMutation(actionKey, filters) {
		return
	}
	switch actionKey {
	case "generate_missing_assets", "review_missing_slots":
		filters.QualityGrade = "missing"
		filters.QualityGradeLabel = generationQualityGradeLabel("missing")
		if actionKey == "generate_missing_assets" {
			filters.RetryableOnly = true
		}
		filters.ExecutionQuality = ""
	}
}
