package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

type listingKitPreviewProjection struct {
	overview            *ListingKitPreviewHeader
	needsReview         bool
	catalog             *catalog.Product
	assets              *asset.Bundle
	assetInventory      *asset.InventorySummary
	assetRenderPreviews []AssetRenderPreview
	platformPreviews    []PlatformAssetRenderPreviews
	generationQueue     *GenerationWorkQueue
	generationOverview  *AssetGenerationOverview
	revisionMeta        *ListingKitRevisionHistoryMeta
	revisionHistory     []ListingKitRevisionRecord
}

func buildListingKitPreviewProjection(result *ListingKitResult, selectedPlatform string) listingKitPreviewProjection {
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	if readProjection == nil {
		return listingKitPreviewProjection{}
	}
	return listingKitPreviewProjection{
		overview:            buildPreviewHeaderFromOverview(readProjection.Overview),
		needsReview:         readProjection.NeedsReview,
		catalog:             readProjection.Attachment.CatalogProduct,
		assets:              readProjection.Attachment.AssetBundle,
		assetInventory:      readProjection.Attachment.AssetInventorySummary,
		assetRenderPreviews: readProjection.Attachment.AssetRenderPreviews,
		platformPreviews:    readProjection.Attachment.PlatformAssetRenderPreviews,
		generationQueue:     readProjection.Attachment.AssetGenerationQueue,
		generationOverview:  readProjection.Attachment.AssetGenerationOverview,
		revisionMeta:        buildRevisionHistoryMeta(result),
		revisionHistory:     buildRevisionHistoryPreviewItems(result.RevisionHistory),
	}
}
