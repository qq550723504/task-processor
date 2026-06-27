package listingkit

import previewdomain "task-processor/internal/listing/preview"

func adaptPreviewDomainResultProjection(
	domainProjection previewdomain.ResultProjection,
	readProjection *listingKitReadProjection,
	revisionHistory []ListingKitRevisionRecord,
) listingKitPreviewProjection {
	projection := listingKitPreviewProjection{
		overview:    adaptPreviewDomainHeader(domainProjection.Overview),
		needsReview: domainProjection.NeedsReview,
		attachment: listingKitPreviewProjectionAttachment{
			catalog:        adaptPreviewDomainCatalog(domainProjection.Attachment),
			assets:         adaptPreviewDomainAssets(domainProjection.Attachment),
			assetInventory: adaptPreviewDomainAssetInventory(domainProjection.Attachment),
		},
		revisionMeta:    adaptPreviewDomainRevisionHistoryMeta(domainProjection.RevisionHistoryMeta),
		revisionHistory: buildRevisionHistoryPreviewItems(revisionHistory),
	}
	if readProjection == nil {
		return projection
	}
	projection.overview = adaptPreviewDomainHeaderWithLegacyPlatformCards(domainProjection.Overview, readProjection.PlatformCards)
	projection.attachment.assetRenderPreviews = readProjection.AssetRenderPreviews
	projection.attachment.platformPreviews = readProjection.PlatformAssetRenderPreviews
	projection.attachment.generationQueue = readProjection.AssetGenerationQueue
	projection.attachment.generationOverview = readProjection.AssetGenerationOverview
	return projection
}
