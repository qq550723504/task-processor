package listingkit

import "strings"

const (
	assetGenerationActionGenerateMissingAssets     = "generate_missing_assets"
	assetGenerationActionUpgradeFallbackAssets     = "upgrade_fallback_assets"
	assetGenerationActionRetryFailedGeneration     = "retry_failed_generation"
	assetGenerationActionReviewReadyAssets         = "review_ready_assets"
	assetGenerationActionContinuePublishReview     = "continue_publish_review"
	assetGenerationActionReviewMissingSlots        = "review_missing_slots"
	assetGenerationActionInspectFailedTasks        = "inspect_failed_renderer_tasks"
	assetGenerationActionRetryProvisionalSlots     = "retry_provisional_slots"
	assetGenerationActionReviewDetailPreviews      = "review_detail_previews"
	assetGenerationActionReviewMeasurementPreviews = "review_measurement_previews"
	assetGenerationActionReviewBadgePreviews       = "review_badge_previews"
	assetGenerationActionReviewCopyPreviews        = "review_copy_previews"
	assetGenerationActionReviewSubjectPreviews     = "review_subject_previews"
	assetGenerationActionRetrySectionGeneration    = "retry_section_generation"
	assetGenerationActionDeferSectionReview        = "defer_section_review"
	assetGenerationActionApproveSectionReview      = "approve_section_review"
)

func allowedAssetGenerationActionKeys() []string {
	return []string{
		assetGenerationActionGenerateMissingAssets,
		assetGenerationActionUpgradeFallbackAssets,
		assetGenerationActionRetryFailedGeneration,
		assetGenerationActionReviewReadyAssets,
		assetGenerationActionContinuePublishReview,
		assetGenerationActionReviewMissingSlots,
		assetGenerationActionInspectFailedTasks,
		assetGenerationActionRetryProvisionalSlots,
		assetGenerationActionReviewDetailPreviews,
		assetGenerationActionReviewMeasurementPreviews,
		assetGenerationActionReviewBadgePreviews,
		assetGenerationActionReviewCopyPreviews,
		assetGenerationActionReviewSubjectPreviews,
		assetGenerationActionRetrySectionGeneration,
		assetGenerationActionDeferSectionReview,
		assetGenerationActionApproveSectionReview,
	}
}

func isAllowedAssetGenerationActionKey(actionKey string) bool {
	actionKey = strings.TrimSpace(actionKey)
	if actionKey == "" {
		return false
	}
	for _, candidate := range allowedAssetGenerationActionKeys() {
		if strings.EqualFold(candidate, actionKey) {
			return true
		}
	}
	return false
}
