package listingkit

import (
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type ListingKitExport struct {
	TaskID           string                `json:"task_id"`
	SelectedPlatform string                `json:"selected_platform,omitempty"`
	Format           string                `json:"format"`
	FileName         string                `json:"file_name"`
	MimeType         string                `json:"mime_type"`
	GeneratedAt      time.Time             `json:"generated_at"`
	Platforms        []string              `json:"platforms,omitempty"`
	CatalogProduct   *catalog.Product      `json:"catalog_product,omitempty"`
	AssetBundle      *asset.Bundle         `json:"asset_bundle,omitempty"`
	Overview         *ListingKitExportMeta `json:"overview,omitempty"`
	Amazon           *AmazonExportPayload  `json:"amazon,omitempty"`
	Shein            *SheinExportPayload   `json:"shein,omitempty"`
	Temu             *TemuExportPayload    `json:"temu,omitempty"`
	Walmart          *WalmartExportPayload `json:"walmart,omitempty"`
}

type ListingKitExportMeta struct {
	Country      string   `json:"country,omitempty"`
	Language     string   `json:"language,omitempty"`
	SourceType   string   `json:"source_type,omitempty"`
	ImageCount   int      `json:"image_count,omitempty"`
	VariantCount int      `json:"variant_count,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type AmazonExportPayload struct {
	Draft *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
}

type SheinExportPayload struct {
	Inspection     *sheinpub.Inspection   `json:"inspection,omitempty"`
	RequestDraft   *sheinpub.RequestDraft `json:"request_draft,omitempty"`
	PreviewProduct *sheinproduct.Product  `json:"preview_product,omitempty"`
	ReviewNotes    []string               `json:"review_notes,omitempty"`
}

type TemuExportPayload struct {
	Package *TemuPackage `json:"package,omitempty"`
}

type WalmartExportPayload struct {
	Package *WalmartPackage `json:"package,omitempty"`
}
