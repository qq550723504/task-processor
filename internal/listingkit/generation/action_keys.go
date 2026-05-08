package generation

import "strings"

const (
	ActionGenerateMissingAssets     = "generate_missing_assets"
	ActionUpgradeFallbackAssets     = "upgrade_fallback_assets"
	ActionRetryFailedGeneration     = "retry_failed_generation"
	ActionReviewReadyAssets         = "review_ready_assets"
	ActionContinuePublishReview     = "continue_publish_review"
	ActionReviewMissingSlots        = "review_missing_slots"
	ActionInspectFailedTasks        = "inspect_failed_renderer_tasks"
	ActionRetryProvisionalSlots     = "retry_provisional_slots"
	ActionReviewDetailPreviews      = "review_detail_previews"
	ActionReviewMeasurementPreviews = "review_measurement_previews"
	ActionReviewBadgePreviews       = "review_badge_previews"
	ActionReviewCopyPreviews        = "review_copy_previews"
	ActionReviewSubjectPreviews     = "review_subject_previews"
	ActionRetrySectionGeneration    = "retry_section_generation"
	ActionDeferSectionReview        = "defer_section_review"
	ActionApproveSectionReview      = "approve_section_review"
)

func AllowedActionKeys() []string {
	return []string{
		ActionGenerateMissingAssets,
		ActionUpgradeFallbackAssets,
		ActionRetryFailedGeneration,
		ActionReviewReadyAssets,
		ActionContinuePublishReview,
		ActionReviewMissingSlots,
		ActionInspectFailedTasks,
		ActionRetryProvisionalSlots,
		ActionReviewDetailPreviews,
		ActionReviewMeasurementPreviews,
		ActionReviewBadgePreviews,
		ActionReviewCopyPreviews,
		ActionReviewSubjectPreviews,
		ActionRetrySectionGeneration,
		ActionDeferSectionReview,
		ActionApproveSectionReview,
	}
}

func IsAllowedActionKey(actionKey string) bool {
	actionKey = strings.TrimSpace(actionKey)
	if actionKey == "" {
		return false
	}
	for _, candidate := range AllowedActionKeys() {
		if strings.EqualFold(candidate, actionKey) {
			return true
		}
	}
	return false
}
