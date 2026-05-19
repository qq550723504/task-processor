package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

const (
	assetGenerationActionGenerateMissingAssets      = listinggeneration.ActionGenerateMissingAssets
	assetGenerationActionUpgradeFallbackAssets      = listinggeneration.ActionUpgradeFallbackAssets
	assetGenerationActionRetryFailedGeneration      = listinggeneration.ActionRetryFailedGeneration
	assetGenerationActionReviewReadyAssets          = listinggeneration.ActionReviewReadyAssets
	assetGenerationActionContinuePublishReview      = listinggeneration.ActionContinuePublishReview
	assetGenerationActionReviewMissingSlots         = listinggeneration.ActionReviewMissingSlots
	assetGenerationActionInspectFailedTasks         = listinggeneration.ActionInspectFailedTasks
	assetGenerationActionRetryProvisionalSlots      = listinggeneration.ActionRetryProvisionalSlots
	assetGenerationActionReviewDetailPreviews       = listinggeneration.ActionReviewDetailPreviews
	assetGenerationActionReviewMeasurementPreviews  = listinggeneration.ActionReviewMeasurementPreviews
	assetGenerationActionReviewBadgePreviews        = listinggeneration.ActionReviewBadgePreviews
	assetGenerationActionReviewCopyPreviews         = listinggeneration.ActionReviewCopyPreviews
	assetGenerationActionReviewSubjectPreviews      = listinggeneration.ActionReviewSubjectPreviews
	assetGenerationActionRetrySectionGeneration     = listinggeneration.ActionRetrySectionGeneration
	assetGenerationActionDeferSectionReview         = listinggeneration.ActionDeferSectionReview
	assetGenerationActionApproveSectionReview       = listinggeneration.ActionApproveSectionReview
	assetGenerationActionRunStandardProductTemporal = "run_standard_product_temporal"
	assetGenerationActionRunPlatformAdaptTemporal   = "run_platform_adapt_temporal"
)

func allowedAssetGenerationActionKeys() []string {
	keys := append([]string(nil), listinggeneration.AllowedActionKeys()...)
	keys = append(keys,
		assetGenerationActionRunStandardProductTemporal,
		assetGenerationActionRunPlatformAdaptTemporal,
	)
	return keys
}

func isAllowedAssetGenerationActionKey(actionKey string) bool {
	if listinggeneration.IsAllowedActionKey(actionKey) {
		return true
	}
	switch actionKey {
	case assetGenerationActionRunStandardProductTemporal, assetGenerationActionRunPlatformAdaptTemporal:
		return true
	default:
		return false
	}
}
