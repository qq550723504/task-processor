package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

type listingKitExportProjection struct {
	attachment listingKitExportProjectionAttachment
	overview   *ListingKitExportMeta
}

type listingKitExportProjectionAttachment struct {
	catalog             *catalog.Product
	assetBundle         *asset.Bundle
	assetInventory      *asset.InventorySummary
	assetRenderPreviews []AssetRenderPreview
	platformPreviews    []PlatformAssetRenderPreviews
	generationQueue     *GenerationWorkQueue
	generationOverview  *AssetGenerationOverview
}

func buildListingKitExportProjection(result *ListingKitResult, selectedPlatform string) listingKitExportProjection {
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	if readProjection == nil {
		return listingKitExportProjection{}
	}
	attachment := readProjection.PreviewInput.Attachment
	return listingKitExportProjection{
		attachment: listingKitExportProjectionAttachment{
			catalog:             attachment.CatalogProduct,
			assetBundle:         attachment.AssetBundle,
			assetInventory:      attachment.AssetInventorySummary,
			assetRenderPreviews: readProjection.AssetRenderPreviews,
			platformPreviews:    readProjection.PlatformAssetRenderPreviews,
			generationQueue:     readProjection.AssetGenerationQueue,
			generationOverview:  readProjection.AssetGenerationOverview,
		},
		overview: buildListingKitExportMetaFromReadProjection(readProjection),
	}
}

func applyListingKitExportProjection(export *ListingKitExport, projection listingKitExportProjection) {
	if export == nil {
		return
	}
	export.CatalogProduct = projection.attachment.catalog
	export.AssetBundle = projection.attachment.assetBundle
	export.AssetInventorySummary = projection.attachment.assetInventory
	export.AssetRenderPreviews = projection.attachment.assetRenderPreviews
	export.PlatformAssetRenderPreviews = projection.attachment.platformPreviews
	export.AssetGenerationQueue = projection.attachment.generationQueue
	export.AssetGenerationOverview = projection.attachment.generationOverview
	export.Overview = projection.overview
}
