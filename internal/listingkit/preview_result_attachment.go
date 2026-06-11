package listingkit

func attachListingKitPreviewResult(preview *ListingKitPreview, result *ListingKitResult, selectedPlatform string) {
	preview.Overview = buildPreviewHeader(result, selectedPlatform)
	preview.NeedsReview = result.Summary != nil && result.Summary.NeedsReview
	preview.Catalog = result.CatalogProduct
	preview.Assets = result.AssetBundle
	preview.AssetInventory = result.AssetInventorySummary
	preview.AssetRenderPreviews = append([]AssetRenderPreview(nil), result.AssetRenderPreviews...)
	preview.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), result.PlatformAssetRenderPreviews...)
	if len(preview.AssetRenderPreviews) == 0 {
		preview.AssetRenderPreviews = buildAssetRenderPreviews(result.AssetBundle)
	}
	if len(preview.PlatformAssetRenderPreviews) == 0 {
		preview.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(result)
	}
	preview.PlatformAssetRenderPreviews = filterPlatformAssetRenderPreviews(preview.PlatformAssetRenderPreviews, selectedPlatform)
	preview.AssetGenerationQueue = result.AssetGenerationQueue
	preview.AssetGenerationOverview = result.AssetGenerationOverview
	preview.RevisionHistoryMeta = buildRevisionHistoryMeta(result)
	preview.RevisionHistory = buildRevisionHistoryPreviewItems(result.RevisionHistory)
}
