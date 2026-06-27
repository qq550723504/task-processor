package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	previewdomain "task-processor/internal/listing/preview"
)

type listingKitPreviewProjection struct {
	overview        *ListingKitPreviewHeader
	needsReview     bool
	attachment      listingKitPreviewProjectionAttachment
	revisionMeta    *ListingKitRevisionHistoryMeta
	revisionHistory []ListingKitRevisionRecord
}

type listingKitPreviewProjectionAttachment struct {
	catalog             *catalog.Product
	assets              *asset.Bundle
	assetInventory      *asset.InventorySummary
	assetRenderPreviews []AssetRenderPreview
	platformPreviews    []PlatformAssetRenderPreviews
	generationQueue     *GenerationWorkQueue
	generationOverview  *AssetGenerationOverview
}

func buildPreviewDomainResultProjection(preview *previewdomain.Preview) previewdomain.ResultProjection {
	return previewdomain.BuildResultProjection(previewdomain.ResultProjectionInput{
		Preview: preview,
	})
}

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
