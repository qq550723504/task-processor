package listingkit

func applyAssetGenerationProvisionalRetryFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	switch actionKey {
	case "upgrade_fallback_assets", "retry_provisional_slots":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = ""
		filters.RetryableOnly = true
		return true
	case "retry_section_generation":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.RetryableOnly = true
		return true
	default:
		return false
	}
}
