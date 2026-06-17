package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildListingKitPreviewReadModelInput(result *ListingKitResult, selectedPlatform string) previewdomain.ReadModelInput {
	return previewdomain.ReadModelInput{
		NeedsReview: calculateListingKitNeedsReview(result),
		Attachment:  buildListingKitPreviewAttachmentInput(result),
		Overview:    buildListingKitPreviewHeaderInput(result, selectedPlatform),
	}
}

func buildListingKitPreviewHeaderInput(result *ListingKitResult, selectedPlatform string) *previewdomain.HeaderInput {
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
	platformCards := buildPlatformPreviewCards(result, selectedPlatform)
	if len(platformCards) > 0 {
		input.PlatformCards = make([]previewdomain.PlatformCard, 0, len(platformCards))
		for _, card := range platformCards {
			input.PlatformCards = append(input.PlatformCards, previewdomain.PlatformCard{
				Platform:              card.Platform,
				Status:                card.Status,
				Summary:               card.Summary,
				NeedsReview:           card.NeedsReview,
				PreviewableItems:      card.PreviewableItems,
				ApprovedSections:      card.ApprovedSections,
				DeferredSections:      card.DeferredSections,
				ReviewPendingSections: card.ReviewPendingSections,
				PrimaryActionKey:      card.PrimaryActionKey,
				PrimaryCTAKind:        card.PrimaryCTAKind,
			})
		}
	}
	return input
}

func buildListingKitPreviewAttachmentInput(result *ListingKitResult) *previewdomain.AttachmentInput {
	if result == nil {
		return nil
	}
	return &previewdomain.AttachmentInput{
		CatalogProduct:        result.CatalogProduct,
		AssetBundle:           result.AssetBundle,
		AssetInventorySummary: result.AssetInventorySummary,
	}
}

func buildListingKitReadProjectionAttachmentExtras(
	result *ListingKitResult,
	selectedPlatform string,
) listingKitReadProjectionAttachmentExtras {
	if result == nil {
		return listingKitReadProjectionAttachmentExtras{}
	}

	assetRenderPreviews := append([]AssetRenderPreview(nil), result.AssetRenderPreviews...)
	if len(assetRenderPreviews) == 0 {
		assetRenderPreviews = buildAssetRenderPreviews(result.AssetBundle)
	}

	platformRenderPreviews := append([]PlatformAssetRenderPreviews(nil), result.PlatformAssetRenderPreviews...)
	if len(platformRenderPreviews) == 0 {
		platformRenderPreviews = buildPlatformAssetRenderPreviews(result)
	}
	platformRenderPreviews = filterPlatformAssetRenderPreviews(platformRenderPreviews, selectedPlatform)

	return listingKitReadProjectionAttachmentExtras{
		AssetRenderPreviews:         assetRenderPreviews,
		PlatformAssetRenderPreviews: platformRenderPreviews,
		AssetGenerationQueue:        result.AssetGenerationQueue,
		AssetGenerationOverview:     result.AssetGenerationOverview,
	}
}
