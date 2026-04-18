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
