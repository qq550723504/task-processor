package listingkit

import (
	"time"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type ListingKitExport struct {
	TaskID                 string                               `json:"task_id"`
	SelectedPlatform       string                               `json:"selected_platform,omitempty"`
	Format                 string                               `json:"format"`
	FileName               string                               `json:"file_name"`
	MimeType               string                               `json:"mime_type"`
	GeneratedAt            time.Time                            `json:"generated_at"`
	Platforms              []string                             `json:"platforms,omitempty"`
	CatalogProduct         *catalog.ProductSnapshot             `json:"catalog_product,omitempty"`
	ApprovedAssetInventory *productasset.ApprovedAssetInventory `json:"approved_asset_inventory,omitempty"`
	Overview               *ListingKitExportMeta                `json:"overview,omitempty"`
	Amazon                 *AmazonExportPayload                 `json:"amazon,omitempty"`
	Shein                  *SheinExportPayload                  `json:"shein,omitempty"`
	Temu                   *TemuExportPayload                   `json:"temu,omitempty"`
	Walmart                *WalmartExportPayload                `json:"walmart,omitempty"`
}

type ListingKitExportMeta struct {
	Country       string                   `json:"country,omitempty"`
	Language      string                   `json:"language,omitempty"`
	SourceType    string                   `json:"source_type,omitempty"`
	ImageCount    int                      `json:"image_count,omitempty"`
	VariantCount  int                      `json:"variant_count,omitempty"`
	Warnings      []string                 `json:"warnings,omitempty"`
	ReviewReasons []string                 `json:"review_reasons,omitempty"`
	PlatformCards []ListingKitPlatformCard `json:"platform_cards,omitempty"`
}
