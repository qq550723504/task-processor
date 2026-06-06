package listingkit

import (
	"strings"

	listinggeneration "task-processor/internal/listingkit/generation"
)

func applyAssetGenerationPreviewCapabilityFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
	spec := listinggeneration.PreviewCapabilityActionSpecForKey(actionKey)
	if spec == nil {
		return false
	}
	filters.ExecutionQuality = ""
	filters.RetryableOnly = false
	filters.RenderPreviewAvailable = true
	filters.PreviewCapability = spec.Capability
	applyAssetGenerationIdealReviewFilters(filters)
	return true
}

func applyAssetGenerationIdealReviewFilters(filters *AssetGenerationRecommendedFilters) {
	if strings.TrimSpace(filters.QualityGrade) != "" {
		return
	}
	filters.QualityGrade = "ideal"
	filters.QualityGradeLabel = generationQualityGradeLabel("ideal")
}

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
