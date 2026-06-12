package listingkit

func initializeListingKitResultAttachment(result *ListingKitResult) *listingKitResultAttachment {
	if result == nil {
		return nil
	}

	return &listingKitResultAttachment{
		CatalogProduct:        result.CatalogProduct,
		AssetBundle:           result.AssetBundle,
		AssetInventorySummary: result.AssetInventorySummary,
		AssetRenderPreviews:   append([]AssetRenderPreview(nil), result.AssetRenderPreviews...),
		PlatformAssetRenderPreviews: append(
			[]PlatformAssetRenderPreviews(nil),
			result.PlatformAssetRenderPreviews...,
		),
		AssetGenerationQueue:    result.AssetGenerationQueue,
		AssetGenerationOverview: result.AssetGenerationOverview,
	}
}

func backfillListingKitResultAttachment(result *ListingKitResult, attachment *listingKitResultAttachment) *listingKitResultAttachment {
	if result == nil || attachment == nil {
		return attachment
	}
	if len(attachment.AssetRenderPreviews) == 0 {
		attachment.AssetRenderPreviews = buildAssetRenderPreviews(result.AssetBundle)
	}
	if len(attachment.PlatformAssetRenderPreviews) == 0 {
		attachment.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(result)
	}
	return attachment
}

func selectListingKitResultAttachmentPlatform(attachment *listingKitResultAttachment, selectedPlatform string) *listingKitResultAttachment {
	if attachment == nil {
		return nil
	}
	attachment.PlatformAssetRenderPreviews = filterPlatformAssetRenderPreviews(attachment.PlatformAssetRenderPreviews, selectedPlatform)
	return attachment
}
