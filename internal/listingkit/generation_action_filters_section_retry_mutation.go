package listingkit

func applyAssetGenerationSectionRetryFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	switch actionKey {
	case "retry_section_generation":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.RetryableOnly = true
		return true
	default:
		return false
	}
}
