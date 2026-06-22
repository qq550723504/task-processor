package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildPlatformPreviewCards(result *ListingKitResult, selectedPlatform string) []ListingKitPlatformCard {
	if result == nil {
		return nil
	}
	queue := result.AssetGenerationQueue
	if queue == nil {
		queue = buildGenerationWorkQueue(result)
	}
	groups := result.PlatformAssetRenderPreviews
	if len(groups) == 0 {
		groups = buildPlatformAssetRenderPreviews(result)
	}
	groups = filterPlatformAssetRenderPreviews(groups, selectedPlatform)
	groupByPlatform := make(map[string]*PlatformAssetRenderPreviews, len(groups))
	for i := range groups {
		group := groups[i]
		groupByPlatform[group.Platform] = &group
	}
	out := make([]ListingKitPlatformCard, 0, 4)
	if selectedPlatform == "" || selectedPlatform == "amazon" {
		if card, ok := buildAmazonPreviewCard(result.Amazon, buildPlatformGenerationWorkQueue(queue, "amazon"), groupByPlatform["amazon"]); ok {
			out = append(out, card)
		}
	}
	if selectedPlatform == "" || selectedPlatform == "shein" {
		if card, ok := buildSheinPreviewCard(result.Shein, buildPlatformGenerationWorkQueue(queue, "shein"), groupByPlatform["shein"]); ok {
			out = append(out, card)
		}
	}
	if selectedPlatform == "" || selectedPlatform == "temu" {
		if card, ok := buildTemuPreviewCard(result.Temu, buildPlatformGenerationWorkQueue(queue, "temu"), groupByPlatform["temu"]); ok {
			out = append(out, card)
		}
	}
	if selectedPlatform == "" || selectedPlatform == "walmart" {
		if card, ok := buildWalmartPreviewCard(result.Walmart, buildPlatformGenerationWorkQueue(queue, "walmart"), groupByPlatform["walmart"]); ok {
			out = append(out, card)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func enrichListingKitPlatformCard(card ListingKitPlatformCard, queue *GenerationWorkQueue, previews *PlatformAssetRenderPreviews) ListingKitPlatformCard {
	previewCapabilityCounts := map[string]int{}
	if previews != nil {
		card.PreviewSummary = clonePlatformAssetRenderPreviewSummary(previews.Summary)
		if previews.Summary != nil {
			card.PreviewableItems = previews.Summary.TotalPreviews
			previewCapabilityCounts = cloneStringIntMap(previews.Summary.CapabilityCounts)
		}
	}
	if queue == nil || queue.Summary == nil {
		card.PreviewCapabilityCounts = cloneStringIntMap(previewCapabilityCounts)
		return card
	}
	summary := queue.Summary
	if summary.PreviewableItems > card.PreviewableItems {
		card.PreviewableItems = summary.PreviewableItems
	}
	for key, value := range summary.PreviewCapabilityCounts {
		previewCapabilityCounts[key] += value
	}
	card.PreviewCapabilityCounts = cloneStringIntMap(previewCapabilityCounts)
	card.QualityGradeCounts = cloneStringIntMap(summary.QualityGradeCounts)
	card.DominantQualityGrade = summary.DominantQualityGrade
	card.DominantQualityGradeLabel = summary.DominantQualityGradeLabel
	card.ApprovedSections = summary.ApprovedSections
	card.DeferredSections = summary.DeferredSections
	card.ReviewPendingSections = summary.ReviewPendingSections
	if summary.MissingItems > 0 || summary.FallbackItems > 0 || summary.FailedItems > 0 || summary.StubbedItems > 0 {
		card.NeedsReview = true
	}
	overview := buildAssetGenerationOverview(queue)
	if overview != nil {
		card.PrimaryActionKey = overview.PrimaryActionKey
		card.PrimaryActionTarget = cloneAssetGenerationActionTarget(overview.PrimaryActionTarget)
		card.RecoverySummary = cloneGenerationRecoverySummary(overview.RecoverySummary)
	}
	applyGenerationRecoveryArbitrationToPlatformCard(&card)
	return card
}

func buildAmazonPreviewCard(pkg *AmazonPackage, queue *GenerationWorkQueue, previews *PlatformAssetRenderPreviews) (ListingKitPlatformCard, bool) {
	if pkg == nil && queue == nil && previews == nil {
		return ListingKitPlatformCard{}, false
	}
	card := ListingKitPlatformCard{
		Platform: "amazon",
		Status:   "ready",
		Summary:  "已生成 Amazon 草稿",
	}
	if pkg != nil && pkg.Draft != nil {
		card.Summary = firstNonEmpty(pkg.Draft.Title, card.Summary)
	}
	return enrichListingKitPlatformCard(card, queue, previews), true
}

func buildSheinPreviewCard(pkg *sheinpub.Package, queue *GenerationWorkQueue, previews *PlatformAssetRenderPreviews) (ListingKitPlatformCard, bool) {
	if pkg == nil && queue == nil && previews == nil {
		return ListingKitPlatformCard{}, false
	}
	card := ListingKitPlatformCard{
		Platform:    "shein",
		Status:      sheinworkspace.BuildPreviewCardStatus(pkg),
		Summary:     sheinworkspace.BuildPreviewCardSummary(pkg),
		NeedsReview: sheinworkspace.PreviewCardNeedsReview(pkg),
	}
	return enrichListingKitPlatformCard(card, queue, previews), true
}

func buildTemuPreviewCard(pkg *TemuPackage, queue *GenerationWorkQueue, previews *PlatformAssetRenderPreviews) (ListingKitPlatformCard, bool) {
	if pkg == nil && queue == nil && previews == nil {
		return ListingKitPlatformCard{}, false
	}
	card := buildReviewNotePreviewCard("temu", "已生成 TEMU 资料包", len(pkg.ReviewNotes) > 0, firstNonEmpty(pkg.GoodsName))
	return enrichListingKitPlatformCard(card, queue, previews), true
}

func buildWalmartPreviewCard(pkg *WalmartPackage, queue *GenerationWorkQueue, previews *PlatformAssetRenderPreviews) (ListingKitPlatformCard, bool) {
	if pkg == nil && queue == nil && previews == nil {
		return ListingKitPlatformCard{}, false
	}
	card := buildReviewNotePreviewCard("walmart", "已生成 Walmart 资料包", len(pkg.ReviewNotes) > 0, firstNonEmpty(pkg.ProductName))
	return enrichListingKitPlatformCard(card, queue, previews), true
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

func buildPlatformGenerationWorkQueue(queue *GenerationWorkQueue, platform string) *GenerationWorkQueue {
	if queue == nil || strings.TrimSpace(platform) == "" {
		return nil
	}
	items := make([]GenerationWorkQueueItem, 0, len(queue.Items))
	for _, item := range queue.Items {
		if item.Platform == platform {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &GenerationWorkQueue{
		Summary: buildGenerationWorkQueueSummary(items),
		Items:   items,
	}
}

func clonePlatformAssetRenderPreviewSummary(summary *PlatformAssetRenderPreviewSummary) *PlatformAssetRenderPreviewSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.CapabilityCounts = cloneStringIntMap(summary.CapabilityCounts)
	cloned.VisualModes = append([]string(nil), summary.VisualModes...)
	return &cloned
}
