package listingkit

func applyAssetGenerationReviewReadyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	switch actionKey {
	case "review_ready_assets", "continue_publish_review":
		applyAssetGenerationIdealReviewFilters(filters)
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
		return true
	case "defer_section_review", "approve_section_review":
		applyAssetGenerationIdealReviewFilters(filters)
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
		return true
	default:
		return false
	}
}
