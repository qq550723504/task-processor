package preview

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

// Attachment captures the preview-facing product and asset data that is already
// platform-neutral and can move with the shared preview shell.
type Attachment struct {
	CatalogProduct        *catalog.Product        `json:"catalog,omitempty"`
	AssetBundle           *asset.Bundle           `json:"assets,omitempty"`
	AssetInventorySummary *asset.InventorySummary `json:"asset_inventory,omitempty"`
}

type AttachmentInput struct {
	CatalogProduct        *catalog.Product
	AssetBundle           *asset.Bundle
	AssetInventorySummary *asset.InventorySummary
}

func BuildAttachment(input AttachmentInput) *Attachment {
	if input.CatalogProduct == nil && input.AssetBundle == nil && input.AssetInventorySummary == nil {
		return nil
	}
	return &Attachment{
		CatalogProduct:        input.CatalogProduct,
		AssetBundle:           input.AssetBundle,
		AssetInventorySummary: input.AssetInventorySummary,
	}
}
