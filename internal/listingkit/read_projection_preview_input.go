package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildListingKitPreviewReadModelInput(result *ListingKitResult, platformCards []ListingKitPlatformCard, selectedPlatform string) previewdomain.ReadModelInput {
	return previewdomain.ReadModelInput{
		NeedsReview: calculateListingKitNeedsReview(result),
		Attachment:  buildListingKitPreviewAttachmentInput(result, selectedPlatform),
		Overview:    buildListingKitPreviewHeaderInput(result, platformCards),
	}
}

func buildListingKitPreviewHeaderInput(result *ListingKitResult, platformCards []ListingKitPlatformCard) *previewdomain.HeaderInput {
	if result == nil {
		return nil
	}

	input := &previewdomain.HeaderInput{
		Country:       result.Country,
		Language:      result.Language,
		StatusMessage: "预览结果已生成",
		ReviewReasons: reviewReasonsFromResult(result),
	}
	if result.Summary != nil {
		input.SourceType = result.Summary.SourceType
		input.ImageCount = result.Summary.ImageCount
		input.VariantCount = result.Summary.VariantCount
		input.Warnings = append([]string(nil), result.Summary.Warnings...)
	}
	if len(platformCards) > 0 {
		input.PlatformCards = buildPreviewDomainPlatformCards(platformCards)
	}
	return input
}

func buildPreviewDomainPlatformCards(platformCards []ListingKitPlatformCard) []previewdomain.PlatformCard {
	cards := make([]previewdomain.PlatformCard, 0, len(platformCards))
	for _, card := range platformCards {
		cards = append(cards, previewdomain.PlatformCard{
			Platform:              card.Platform,
			Status:                card.Status,
			Summary:               card.Summary,
			NeedsReview:           card.NeedsReview,
			PreviewableItems:      card.PreviewableItems,
			ApprovedSections:      card.ApprovedSections,
			DeferredSections:      card.DeferredSections,
			ReviewPendingSections: card.ReviewPendingSections,
		})
	}
	return cards
}

func buildListingKitPreviewAttachmentInput(result *ListingKitResult, _ string) *previewdomain.AttachmentInput {
	if result == nil {
		return nil
	}
	return &previewdomain.AttachmentInput{
		CatalogProduct:         result.CatalogProduct,
		ApprovedAssetInventory: cloneApprovedAssetInventory(result.ApprovedAssetInventory),
	}
}

func buildListingKitReadProjectionAttachmentExtras(
	result *ListingKitResult,
	_ string,
) listingKitReadProjectionAttachmentExtras {
	if result == nil {
		return listingKitReadProjectionAttachmentExtras{}
	}

	return listingKitReadProjectionAttachmentExtras{}
}
