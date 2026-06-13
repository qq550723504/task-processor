package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

type listingKitExportProjection struct {
	catalog             *catalog.Product
	assetBundle         *asset.Bundle
	assetInventory      *asset.InventorySummary
	assetRenderPreviews []AssetRenderPreview
	platformPreviews    []PlatformAssetRenderPreviews
	generationQueue     *GenerationWorkQueue
	generationOverview  *AssetGenerationOverview
	overview            *ListingKitExportMeta
}

func buildListingKitExportProjection(result *ListingKitResult, selectedPlatform string) listingKitExportProjection {
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	if readProjection == nil {
		return listingKitExportProjection{}
	}
	return listingKitExportProjection{
		catalog:             readProjection.Attachment.CatalogProduct,
		assetBundle:         readProjection.Attachment.AssetBundle,
		assetInventory:      readProjection.Attachment.AssetInventorySummary,
		assetRenderPreviews: readProjection.Attachment.AssetRenderPreviews,
		platformPreviews:    readProjection.Attachment.PlatformAssetRenderPreviews,
		generationQueue:     readProjection.Attachment.AssetGenerationQueue,
		generationOverview:  readProjection.Attachment.AssetGenerationOverview,
		overview:            buildListingKitExportMetaFromReadProjection(readProjection),
	}
}
