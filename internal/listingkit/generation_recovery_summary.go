package listingkit

import "strings"

func buildGenerationRecoverySummaryFromQueue(queue *GenerationWorkQueue) *GenerationRecoverySummary {
	if queue == nil {
		return nil
	}
	descriptors := buildGenerationQueueResponseDescriptors(&GenerationQueuePage{Items: queue.Items})
	return buildGenerationRecoverySummaryFromDescriptors(descriptors)
}

func buildGenerationRecoverySummaryFromDescriptors(items []GenerationPanelResourceDescriptor) *GenerationRecoverySummary {
	primary, recommended := selectGenerationPanelRecoveryDescriptors(items)
	if primary == nil {
		return nil
	}
	profile := generationRecoveryProfileForHint(primary.RecoveryHint)
	return &GenerationRecoverySummary{
		Title:                  profile.Title,
		Summary:                profile.Summary,
		Severity:               primary.RecoverySeverity,
		Urgency:                primary.RecoveryUrgency,
		CTAKind:                primary.RecoveryCTAKind,
		ActionKey:              primary.RecoveryActionKey,
		RecommendedCount:       len(recommended),
		PrimaryDescriptor:      cloneGenerationPanelResourceDescriptor(primary),
		RecommendedDescriptors: cloneGenerationPanelResourceDescriptors(recommended),
	}
}

func applyGenerationRecoverySummaryToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {
	if page == nil {
		return nil
	}
	page.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(page.ResourceDescriptors)
	return page
}

func applyGenerationRecoverySummaryToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {
	if response == nil {
		return nil
	}
	response.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors)
	return response
}

func applyGenerationRecoverySummaryToReviewPreviewResponse(response *GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {
	if response == nil {
		return nil
	}
	response.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors)
	return response
}

func applyGenerationRecoverySummaryToActionResult(result *GenerationActionExecutionResult) *GenerationActionExecutionResult {
	if result == nil {
		return nil
	}
	result.RecoverySummary = buildGenerationRecoverySummaryFromDescriptors(result.ResourceDescriptors)
	return result
}

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
