package listingkit

import previewdomain "task-processor/internal/listing/preview"

func calculateListingKitNeedsReview(result *ListingKitResult) bool {
	return result != nil && result.Summary != nil && result.Summary.NeedsReview
}

func assembleListingKitReadProjection(
	previewInput previewdomain.ReadModelInput,
	platformCards []ListingKitPlatformCard,
	attachmentExtras listingKitReadProjectionAttachmentExtras,
	revisionMeta *previewdomain.RevisionHistoryMetaInput,
) *listingKitReadProjection {
	previewInput.RevisionHistoryMeta = revisionMeta
	return &listingKitReadProjection{
		PreviewInput:                previewInput,
		PlatformCards:               platformCards,
		AssetRenderPreviews:         attachmentExtras.AssetRenderPreviews,
		PlatformAssetRenderPreviews: attachmentExtras.PlatformAssetRenderPreviews,
		AssetGenerationQueue:        attachmentExtras.AssetGenerationQueue,
		AssetGenerationOverview:     attachmentExtras.AssetGenerationOverview,
	}
}
