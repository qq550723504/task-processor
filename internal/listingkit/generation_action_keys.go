package listingkit

import "task-processor/internal/listingkit/generation"

const (
	assetGenerationActionGenerateMissingAssets      = generation.ActionGenerateMissingAssets
	assetGenerationActionUpgradeFallbackAssets      = generation.ActionUpgradeFallbackAssets
	assetGenerationActionRetryFailedGeneration      = generation.ActionRetryFailedGeneration
	assetGenerationActionReviewReadyAssets          = generation.ActionReviewReadyAssets
	assetGenerationActionContinuePublishReview      = generation.ActionContinuePublishReview
	assetGenerationActionReviewMissingSlots         = generation.ActionReviewMissingSlots
	assetGenerationActionInspectFailedTasks         = generation.ActionInspectFailedTasks
	assetGenerationActionRetryProvisionalSlots      = generation.ActionRetryProvisionalSlots
	assetGenerationActionReviewDetailPreviews       = generation.ActionReviewDetailPreviews
	assetGenerationActionReviewMeasurementPreviews  = generation.ActionReviewMeasurementPreviews
	assetGenerationActionReviewBadgePreviews        = generation.ActionReviewBadgePreviews
	assetGenerationActionReviewCopyPreviews         = generation.ActionReviewCopyPreviews
	assetGenerationActionReviewSubjectPreviews      = generation.ActionReviewSubjectPreviews
	assetGenerationActionRetrySectionGeneration     = generation.ActionRetrySectionGeneration
	assetGenerationActionDeferSectionReview         = generation.ActionDeferSectionReview
	assetGenerationActionApproveSectionReview       = generation.ActionApproveSectionReview
	assetGenerationActionRunStandardProductTemporal = "run_standard_product_temporal"
	assetGenerationActionRunPlatformAdaptTemporal   = "run_platform_adapt_temporal"
)

func allowedAssetGenerationActionKeys() []string {
	keys := append([]string(nil), generation.AllowedActionKeys()...)
	keys = append(keys,
		assetGenerationActionRunStandardProductTemporal,
		assetGenerationActionRunPlatformAdaptTemporal,
	)
	return keys
}

func isAllowedAssetGenerationActionKey(actionKey string) bool {
	if generation.IsAllowedActionKey(actionKey) {
		return true
	}
	switch actionKey {
	case assetGenerationActionRunStandardProductTemporal, assetGenerationActionRunPlatformAdaptTemporal:
		return true
	default:
		return false
	}
}
