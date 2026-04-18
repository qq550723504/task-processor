package listingkit

import "strings"

func applyGenerationRecoveryArbitrationToOverview(overview *AssetGenerationOverview) *AssetGenerationOverview {
	if overview == nil {
		return nil
	}
	overview.RecoverySummary = cloneGenerationRecoverySummary(overview.RecoverySummary)
	overview.PrimaryCTAKind = "generation_action"
	if overview.PrimaryActionTarget != nil {
		overview.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(overview.PrimaryActionTarget.NavigationTarget)
	}
	if !shouldPreferRecoveryAsPrimaryCTA(overview.PrimaryActionKey, overview.RecoverySummary) {
		return overview
	}
	overview.PrimaryCTAKind = "recovery"
	overview.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(overview.RecoverySummary.PrimaryDescriptor.RecoveryTarget)
	if strings.TrimSpace(overview.PrimaryActionReason) == "" {
		overview.PrimaryActionReason = overview.RecoverySummary.Summary
	} else if !strings.Contains(overview.PrimaryActionReason, overview.RecoverySummary.Summary) {
		overview.PrimaryActionReason = overview.RecoverySummary.Summary + " " + overview.PrimaryActionReason
	}
	overview.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromOverview(overview)
	return overview
}

func finalizeGenerationOverviewActionSummary(overview *AssetGenerationOverview) *AssetGenerationOverview {
	if overview == nil {
		return nil
	}
	overview.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromOverview(overview)
	return overview
}

func applyGenerationRecoveryArbitrationToPlatformCard(card *ListingKitPlatformCard) {
	if card == nil {
		return
	}
	card.RecoverySummary = cloneGenerationRecoverySummary(card.RecoverySummary)
	card.PrimaryCTAKind = "generation_action"
	if card.PrimaryActionTarget != nil {
		card.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(card.PrimaryActionTarget.NavigationTarget)
	}
	if !shouldPreferRecoveryAsPrimaryCTA(card.PrimaryActionKey, card.RecoverySummary) {
		card.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromPlatformCard(card)
		return
	}
	card.PrimaryCTAKind = "recovery"
	card.PrimaryNavigationTarget = cloneGenerationReviewNavigationTarget(card.RecoverySummary.PrimaryDescriptor.RecoveryTarget)
	card.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromPlatformCard(card)
}

func shouldPreferRecoveryAsPrimaryCTA(primaryActionKey string, summary *GenerationRecoverySummary) bool {
	if summary == nil || summary.PrimaryDescriptor == nil || summary.PrimaryDescriptor.RecoveryTarget == nil {
		return false
	}
	if strings.TrimSpace(summary.Urgency) != "now" {
		return false
	}
	switch strings.TrimSpace(primaryActionKey) {
	case "",
		assetGenerationActionReviewReadyAssets,
		assetGenerationActionContinuePublishReview,
		assetGenerationActionReviewDetailPreviews,
		assetGenerationActionReviewMeasurementPreviews,
		assetGenerationActionReviewBadgePreviews,
		assetGenerationActionReviewCopyPreviews,
		assetGenerationActionReviewSubjectPreviews:
		return true
	default:
		return false
	}
}
