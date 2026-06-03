package listingkit

import (
	"strings"

	listinggeneration "task-processor/internal/listingkit/generation"
)

func applyAssetGenerationActionFiltersMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {
	if filters == nil {
		return
	}
	if applyAssetGenerationPreviewCapabilityFilters(actionKey, filters) {
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
	case "retry_failed_generation", "inspect_failed_renderer_tasks":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = "failed"
		filters.RetryableOnly = true
	case "upgrade_fallback_assets", "retry_provisional_slots":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.ExecutionQuality = ""
		filters.RetryableOnly = true
	case "review_ready_assets", "continue_publish_review":
		applyAssetGenerationIdealReviewFilters(filters)
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
	case "retry_section_generation":
		filters.QualityGrade = "provisional"
		filters.QualityGradeLabel = generationQualityGradeLabel("provisional")
		filters.RetryableOnly = true
	case "defer_section_review", "approve_section_review":
		applyAssetGenerationIdealReviewFilters(filters)
		filters.ExecutionQuality = ""
		filters.RetryableOnly = false
	}
}

func applyAssetGenerationPreviewCapabilityFilters(actionKey string, filters *AssetGenerationRecommendedFilters) bool {
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
