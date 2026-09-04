package preview

import (
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

// Attachment captures the preview-facing product and asset data that is already
// platform-neutral and can move with the shared preview shell.
type Attachment struct {
	CatalogProduct         *catalog.ProductSnapshot             `json:"catalog,omitempty"`
	ApprovedAssetInventory *productasset.ApprovedAssetInventory `json:"approved_asset_inventory,omitempty"`
}

type AttachmentInput struct {
	CatalogProduct         *catalog.ProductSnapshot
	ApprovedAssetInventory *productasset.ApprovedAssetInventory
}

func BuildAttachment(input AttachmentInput) *Attachment {
	if input.CatalogProduct == nil && input.ApprovedAssetInventory == nil {
		return nil
	}
	return &Attachment{
		CatalogProduct:         input.CatalogProduct,
		ApprovedAssetInventory: input.ApprovedAssetInventory,
	}
}
