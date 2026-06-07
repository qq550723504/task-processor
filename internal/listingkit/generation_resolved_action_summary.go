package listingkit

func buildGenerationResolvedActionSummaryFromOverview(overview *AssetGenerationOverview) *GenerationResolvedActionSummary {
	if overview == nil {
		return nil
	}
	summary := &GenerationResolvedActionSummary{
		SourceKind:       overview.PrimaryCTAKind,
		Title:            overview.PrimaryAction,
		Summary:          overview.PrimaryActionReason,
		CTAKind:          overview.PrimaryCTAKind,
		ActionKey:        overview.PrimaryActionKey,
		NavigationTarget: cloneGenerationReviewNavigationTarget(overview.PrimaryNavigationTarget),
		ActionTarget:     cloneAssetGenerationActionTarget(overview.PrimaryActionTarget),
		RecoverySummary:  cloneGenerationRecoverySummary(overview.RecoverySummary),
	}
	if summary.SourceKind == "recovery" && overview.RecoverySummary != nil {
		summary.Title = overview.RecoverySummary.Title
		summary.Summary = overview.RecoverySummary.Summary
		summary.CTAKind = overview.RecoverySummary.CTAKind
		summary.ActionKey = overview.RecoverySummary.ActionKey
		summary.NavigationTarget = cloneGenerationReviewNavigationTarget(overview.PrimaryNavigationTarget)
		summary.ActionTarget = nil
	}
	if isEmptyGenerationResolvedActionSummary(summary) {
		return nil
	}
	return summary
}

func buildGenerationResolvedActionSummaryFromPlatformCard(card *ListingKitPlatformCard) *GenerationResolvedActionSummary {
	if card == nil {
		return nil
	}
	summary := &GenerationResolvedActionSummary{
		SourceKind:       card.PrimaryCTAKind,
		Title:            card.Summary,
		Summary:          card.Summary,
		CTAKind:          card.PrimaryCTAKind,
		ActionKey:        card.PrimaryActionKey,
		NavigationTarget: cloneGenerationReviewNavigationTarget(card.PrimaryNavigationTarget),
		ActionTarget:     cloneAssetGenerationActionTarget(card.PrimaryActionTarget),
		RecoverySummary:  cloneGenerationRecoverySummary(card.RecoverySummary),
	}
	if summary.SourceKind == "recovery" && card.RecoverySummary != nil {
		summary.Title = card.RecoverySummary.Title
		summary.Summary = card.RecoverySummary.Summary
		summary.CTAKind = card.RecoverySummary.CTAKind
		summary.ActionKey = card.RecoverySummary.ActionKey
		summary.NavigationTarget = cloneGenerationReviewNavigationTarget(card.PrimaryNavigationTarget)
		summary.ActionTarget = nil
	}
	if isEmptyGenerationResolvedActionSummary(summary) {
		return nil
	}
	return summary
}

func isEmptyGenerationResolvedActionSummary(summary *GenerationResolvedActionSummary) bool {
	if summary == nil {
		return true
	}
	return summary.Title == "" &&
		summary.Summary == "" &&
		summary.CTAKind == "" &&
		summary.ActionKey == "" &&
		summary.NavigationTarget == nil &&
		summary.ActionTarget == nil &&
		summary.RecoverySummary == nil
}

func cloneGenerationResolvedActionSummary(summary *GenerationResolvedActionSummary) *GenerationResolvedActionSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(summary.NavigationTarget)
	cloned.ActionTarget = cloneAssetGenerationActionTarget(summary.ActionTarget)
	cloned.RecoverySummary = cloneGenerationRecoverySummary(summary.RecoverySummary)
	return &cloned
}

func applyGenerationResolvedActionSummaryToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {
	if page == nil {
		return nil
	}
	page.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromRecovery(page.RecoverySummary)
	return page
}

func applyGenerationResolvedActionSummaryToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {
	if response == nil {
		return nil
	}
	var target *GenerationReviewTarget
	if response.Session != nil {
		target = response.Session.FocusedTarget
		if target == nil {
			target = response.Session.DefaultTarget
		}
	}
	response.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromReviewTarget(response.RecoverySummary, target)
	return response
}

func applyGenerationResolvedActionSummaryToReviewPreviewResponse(response *GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {
	if response == nil {
		return nil
	}
	response.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromReviewTarget(response.RecoverySummary, response.ReviewTarget)
	return response
}

func applyGenerationResolvedActionSummaryToActionResult(result *GenerationActionExecutionResult) *GenerationActionExecutionResult {
	if result == nil {
		return nil
	}
	if result.Overview != nil && result.Overview.ResolvedActionSummary != nil {
		result.ResolvedActionSummary = cloneGenerationResolvedActionSummary(result.Overview.ResolvedActionSummary)
		return result
	}
	result.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromActionTarget(result.RecoverySummary, result.ResolvedTarget)
	return result
}

func applyGenerationResolvedActionSummaryToNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}
	switch {
	case response.Action != nil && response.Action.ResolvedActionSummary != nil:
		response.ResolvedActionSummary = cloneGenerationResolvedActionSummary(response.Action.ResolvedActionSummary)
	case response.ReviewSession != nil && response.ReviewSession.ResolvedActionSummary != nil:
		response.ResolvedActionSummary = cloneGenerationResolvedActionSummary(response.ReviewSession.ResolvedActionSummary)
	case response.ReviewPreview != nil && response.ReviewPreview.ResolvedActionSummary != nil:
		response.ResolvedActionSummary = cloneGenerationResolvedActionSummary(response.ReviewPreview.ResolvedActionSummary)
	case response.Queue != nil && response.Queue.ResolvedActionSummary != nil:
		response.ResolvedActionSummary = cloneGenerationResolvedActionSummary(response.Queue.ResolvedActionSummary)
	default:
		response.ResolvedActionSummary = buildGenerationResolvedActionSummaryFromRecovery(buildGenerationRecoverySummaryFromDescriptors(response.ResourceDescriptors))
	}
	return response
}

func buildGenerationResolvedActionSummaryFromRecovery(summary *GenerationRecoverySummary) *GenerationResolvedActionSummary {
	if summary == nil {
		return nil
	}
	navigation := cloneGenerationReviewNavigationTarget(nil)
	if summary.PrimaryDescriptor != nil {
		navigation = cloneGenerationReviewNavigationTarget(summary.PrimaryDescriptor.RecoveryTarget)
	}
	resolved := &GenerationResolvedActionSummary{
		SourceKind:       "recovery",
		Title:            summary.Title,
		Summary:          summary.Summary,
		CTAKind:          summary.CTAKind,
		ActionKey:        summary.ActionKey,
		NavigationTarget: navigation,
		RecoverySummary:  cloneGenerationRecoverySummary(summary),
	}
	if isEmptyGenerationResolvedActionSummary(resolved) {
		return nil
	}
	return resolved
}

func buildGenerationResolvedActionSummaryFromActionTarget(summary *GenerationRecoverySummary, target *AssetGenerationActionTarget) *GenerationResolvedActionSummary {
	if target == nil {
		return buildGenerationResolvedActionSummaryFromRecovery(summary)
	}
	navigation := cloneGenerationReviewNavigationTarget(target.NavigationTarget)
	if navigation == nil {
		navigation = buildGenerationReviewActionNavigationTarget(target)
	}
	resolved := &GenerationResolvedActionSummary{
		SourceKind:       "generation_action",
		Title:            generationResolvedActionTitle(target.ActionKey),
		Summary:          generationResolvedActionReason(target.ActionKey),
		CTAKind:          "generation_action",
		ActionKey:        target.ActionKey,
		NavigationTarget: navigation,
		ActionTarget:     cloneAssetGenerationActionTarget(target),
		RecoverySummary:  cloneGenerationRecoverySummary(summary),
	}
	if isEmptyGenerationResolvedActionSummary(resolved) {
		return nil
	}
	return resolved
}

func buildGenerationResolvedActionSummaryFromReviewTarget(summary *GenerationRecoverySummary, target *GenerationReviewTarget) *GenerationResolvedActionSummary {
	if target == nil {
		return buildGenerationResolvedActionSummaryFromRecovery(summary)
	}
	resolved := &GenerationResolvedActionSummary{
		SourceKind:       "review_target",
		Title:            reviewActionLabelForCapability(target.Capability),
		Summary:          "Review the current section and preview focus.",
		CTAKind:          "review",
		ActionKey:        target.ActionKey,
		NavigationTarget: cloneGenerationReviewNavigationTarget(target.NavigationTarget),
		RecoverySummary:  cloneGenerationRecoverySummary(summary),
	}
	if isEmptyGenerationResolvedActionSummary(resolved) {
		return nil
	}
	return resolved
}

func generationResolvedActionTitle(actionKey string) string {
	switch actionKey {
	case assetGenerationActionGenerateMissingAssets:
		return "Generate Missing Assets"
	case assetGenerationActionUpgradeFallbackAssets:
		return "Upgrade Fallback Assets"
	case assetGenerationActionRetryFailedGeneration:
		return "Retry Failed Generation"
	case assetGenerationActionContinuePublishReview:
		return "Continue Publish Review"
	case assetGenerationActionReviewReadyAssets:
		return "Review Ready Assets"
	case assetGenerationActionRetrySectionGeneration:
		return "Retry Section Generation"
	default:
		return "Review Generation Action"
	}
}

func generationResolvedActionReason(actionKey string) string {
	switch actionKey {
	case assetGenerationActionGenerateMissingAssets:
		return "Required slots are still missing assets."
	case assetGenerationActionUpgradeFallbackAssets:
		return "Fallback assets still cover publish-critical slots."
	case assetGenerationActionRetryFailedGeneration:
		return "Some generation steps failed and should be retried."
	case assetGenerationActionContinuePublishReview:
		return "Asset coverage is ready to continue publish review."
	case assetGenerationActionReviewReadyAssets:
		return "Review the current ready assets and sidecars."
	case assetGenerationActionRetrySectionGeneration:
		return "Retry the current section generation path."
	default:
		return "Review the current generation action."
	}
}
