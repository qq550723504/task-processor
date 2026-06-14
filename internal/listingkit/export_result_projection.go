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
	attachment := readProjection.PreviewInput.Attachment
	return listingKitExportProjection{
		catalog:             attachment.CatalogProduct,
		assetBundle:         attachment.AssetBundle,
		assetInventory:      attachment.AssetInventorySummary,
		assetRenderPreviews: readProjection.AssetRenderPreviews,
		platformPreviews:    readProjection.PlatformAssetRenderPreviews,
		generationQueue:     readProjection.AssetGenerationQueue,
		generationOverview:  readProjection.AssetGenerationOverview,
		overview:            buildListingKitExportMetaFromReadProjection(readProjection),
	}
}
