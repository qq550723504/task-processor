package listingkit

func attachListingKitPreviewResult(preview *ListingKitPreview, result *ListingKitResult, selectedPlatform string) {
	attachment := buildListingKitResultAttachment(result, selectedPlatform)

	preview.Overview = buildPreviewHeader(result, selectedPlatform)
	preview.NeedsReview = result.Summary != nil && result.Summary.NeedsReview
	preview.Catalog = attachment.CatalogProduct
	preview.Assets = attachment.AssetBundle
	preview.AssetInventory = attachment.AssetInventorySummary
	preview.AssetRenderPreviews = attachment.AssetRenderPreviews
	preview.PlatformAssetRenderPreviews = attachment.PlatformAssetRenderPreviews
	preview.AssetGenerationQueue = attachment.AssetGenerationQueue
	preview.AssetGenerationOverview = attachment.AssetGenerationOverview
	preview.RevisionHistoryMeta = buildRevisionHistoryMeta(result)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(result.RevisionHistory)
}
