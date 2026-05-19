package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func buildSheinPreviewCard(pkg *sheinpub.Package, queue *GenerationWorkQueue, previews *PlatformAssetRenderPreviews) (ListingKitPlatformCard, bool) {
	if pkg == nil && queue == nil && previews == nil {
		return ListingKitPlatformCard{}, false
	}
	card := ListingKitPlatformCard{
		Platform:    "shein",
		Status:      buildSheinPreviewCardStatus(pkg),
		Summary:     buildSheinPreviewCardSummary(pkg),
		NeedsReview: sheinPreviewCardNeedsReview(pkg),
	}
	return enrichListingKitPlatformCard(card, queue, previews), true
}

func buildSheinPreviewCardStatus(pkg *sheinpub.Package) string {
	if pkg != nil && pkg.Inspection != nil && pkg.Inspection.NeedsReview {
		return "needs_review"
	}
	return "ready"
}

func buildSheinPreviewCardSummary(pkg *sheinpub.Package) string {
	summary := "已生成 SHEIN 预览"
	if pkg != nil {
		summary = firstNonEmpty(pkg.SpuName, pkg.ProductNameEn, summary)
	}
	if pkg != nil && pkg.Inspection != nil {
		summary = firstNonEmpty(joinStrings(pkg.Inspection.Summary, "；"), summary)
	}
	return summary
}

func sheinPreviewCardNeedsReview(pkg *sheinpub.Package) bool {
	if pkg == nil {
		return false
	}
	if len(pkg.ReviewNotes) > 0 {
		return true
	}
	return pkg.Inspection != nil && pkg.Inspection.NeedsReview
}

func buildReviewNotePreviewCard(platform string, fallbackSummary string, needsReview bool, summaryCandidates ...string) ListingKitPlatformCard {
	card := ListingKitPlatformCard{
		Platform:    platform,
		Status:      "ready",
		Summary:     fallbackSummary,
		NeedsReview: needsReview,
	}
	if needsReview {
		card.Status = "needs_review"
	}
	card.Summary = firstNonEmpty(append(summaryCandidates, fallbackSummary)...)
	return card
}
