package listingkit

func applyAssetGenerationFailedRetryFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	switch actionKey {
	case "retry_failed_generation", "inspect_failed_renderer_tasks":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = "failed"
		filters.RetryableOnly = true
		return true
	default:
		return false
	}
}
