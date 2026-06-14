package listingkit

func attachListingKitPreviewResult(preview *ListingKitPreview, task *Task, selectedPlatform string) {
	if preview == nil {
		return
	}
	projection := buildListingKitPreviewProjection(task, selectedPlatform)
	preview.Overview = projection.overview
	preview.NeedsReview = projection.needsReview
	preview.Catalog = projection.catalog
	preview.Assets = projection.assets
	preview.AssetInventory = projection.assetInventory
	preview.AssetRenderPreviews = projection.assetRenderPreviews
	preview.PlatformAssetRenderPreviews = projection.platformPreviews
	preview.AssetGenerationQueue = projection.generationQueue
	preview.AssetGenerationOverview = projection.generationOverview
	preview.RevisionHistoryMeta = projection.revisionMeta
	preview.RevisionHistory = projection.revisionHistory
}
