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
