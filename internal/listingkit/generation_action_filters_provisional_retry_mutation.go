package listingkit

func applyAssetGenerationProvisionalRetryFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	if applyAssetGenerationSectionRetryFilterMutation(actionKey, filters) {
		return true
	}
	switch actionKey {
	case "upgrade_fallback_assets", "retry_provisional_slots":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = ""
		filters.RetryableOnly = true
		return true
	default:
		return false
	}
}
