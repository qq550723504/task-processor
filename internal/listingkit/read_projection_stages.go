package listingkit

import previewdomain "task-processor/internal/listing/preview"

func calculateListingKitNeedsReview(result *ListingKitResult) bool {
	return result != nil && result.Summary != nil && result.Summary.NeedsReview
}

func assembleListingKitReadProjection(
	previewInput previewdomain.ReadModelInput,
	platformCards []ListingKitPlatformCard,
	assetRenderPreviews []AssetRenderPreview,
	platformAssetRenderPreviews []PlatformAssetRenderPreviews,
	assetGenerationQueue *GenerationWorkQueue,
	assetGenerationOverview *AssetGenerationOverview,
	revisionMeta *previewdomain.RevisionHistoryMetaInput,
) *listingKitReadProjection {
	previewInput.RevisionHistoryMeta = revisionMeta
	return &listingKitReadProjection{
		PreviewInput:                previewInput,
		PlatformCards:               platformCards,
		AssetRenderPreviews:         assetRenderPreviews,
		PlatformAssetRenderPreviews: platformAssetRenderPreviews,
		AssetGenerationQueue:        assetGenerationQueue,
		AssetGenerationOverview:     assetGenerationOverview,
	}
}
