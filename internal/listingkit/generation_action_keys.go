package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

const (
	assetGenerationActionGenerateMissingAssets     = listinggeneration.ActionGenerateMissingAssets
	assetGenerationActionUpgradeFallbackAssets     = listinggeneration.ActionUpgradeFallbackAssets
	assetGenerationActionRetryFailedGeneration     = listinggeneration.ActionRetryFailedGeneration
	assetGenerationActionReviewReadyAssets         = listinggeneration.ActionReviewReadyAssets
	assetGenerationActionContinuePublishReview     = listinggeneration.ActionContinuePublishReview
	assetGenerationActionReviewMissingSlots        = listinggeneration.ActionReviewMissingSlots
	assetGenerationActionInspectFailedTasks        = listinggeneration.ActionInspectFailedTasks
	assetGenerationActionRetryProvisionalSlots     = listinggeneration.ActionRetryProvisionalSlots
	assetGenerationActionReviewDetailPreviews      = listinggeneration.ActionReviewDetailPreviews
	assetGenerationActionReviewMeasurementPreviews = listinggeneration.ActionReviewMeasurementPreviews
	assetGenerationActionReviewBadgePreviews       = listinggeneration.ActionReviewBadgePreviews
	assetGenerationActionReviewCopyPreviews        = listinggeneration.ActionReviewCopyPreviews
	assetGenerationActionReviewSubjectPreviews     = listinggeneration.ActionReviewSubjectPreviews
	assetGenerationActionRetrySectionGeneration    = listinggeneration.ActionRetrySectionGeneration
	assetGenerationActionDeferSectionReview        = listinggeneration.ActionDeferSectionReview
	assetGenerationActionApproveSectionReview      = listinggeneration.ActionApproveSectionReview
)

func allowedAssetGenerationActionKeys() []string {
	return listinggeneration.AllowedActionKeys()
}

func isAllowedAssetGenerationActionKey(actionKey string) bool {
	return listinggeneration.IsAllowedActionKey(actionKey)
}
