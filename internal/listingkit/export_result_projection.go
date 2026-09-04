package listingkit

import (
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type listingKitExportProjection struct {
	attachment listingKitExportProjectionAttachment
	overview   *ListingKitExportMeta
}

type listingKitExportProjectionAttachment struct {
	catalog        *catalog.ProductSnapshot
	approvedAssets *productasset.ApprovedAssetInventory
}

func buildListingKitExportProjection(result *ListingKitResult, selectedPlatform string) listingKitExportProjection {
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	if readProjection == nil {
		return listingKitExportProjection{}
	}
	attachment := readProjection.PreviewInput.Attachment
	return listingKitExportProjection{
		attachment: listingKitExportProjectionAttachment{
			catalog:        attachment.CatalogProduct,
			approvedAssets: attachment.ApprovedAssetInventory,
		},
		overview: buildListingKitExportMetaFromReadProjection(readProjection),
	}
}

func applyListingKitExportProjection(export *ListingKitExport, projection listingKitExportProjection) {
	if export == nil {
		return
	}
	export.CatalogProduct = projection.attachment.catalog
	export.ApprovedAssetInventory = cloneApprovedAssetInventory(projection.attachment.approvedAssets)
	export.Overview = projection.overview
}
