package listingkit

func attachListingKitPreviewResult(preview *ListingKitPreview, result *ListingKitResult, selectedPlatform string) {
	projection := buildListingKitReadProjection(result, selectedPlatform)
	preview.Overview = buildPreviewHeaderFromOverview(projection.Overview)
	preview.NeedsReview = projection.NeedsReview
	preview.Catalog = projection.Attachment.CatalogProduct
	preview.Assets = projection.Attachment.AssetBundle
	preview.AssetInventory = projection.Attachment.AssetInventorySummary
	preview.AssetRenderPreviews = projection.Attachment.AssetRenderPreviews
	preview.PlatformAssetRenderPreviews = projection.Attachment.PlatformAssetRenderPreviews
	preview.AssetGenerationQueue = projection.Attachment.AssetGenerationQueue
	preview.AssetGenerationOverview = projection.Attachment.AssetGenerationOverview
	preview.RevisionHistoryMeta = buildRevisionHistoryMeta(result)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(result.RevisionHistory)
}
