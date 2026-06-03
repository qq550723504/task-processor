package listingkit

func applyAssetGenerationMissingSlotFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
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
