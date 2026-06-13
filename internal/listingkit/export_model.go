package listingkit

import (
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
)

type ListingKitExport struct {
	TaskID                      string                        `json:"task_id"`
	SelectedPlatform            string                        `json:"selected_platform,omitempty"`
	Format                      string                        `json:"format"`
	FileName                    string                        `json:"file_name"`
	MimeType                    string                        `json:"mime_type"`
	GeneratedAt                 time.Time                     `json:"generated_at"`
	Platforms                   []string                      `json:"platforms,omitempty"`
	CatalogProduct              *catalog.Product              `json:"catalog_product,omitempty"`
	AssetBundle                 *asset.Bundle                 `json:"asset_bundle,omitempty"`
	AssetInventorySummary       *asset.InventorySummary       `json:"asset_inventory_summary,omitempty"`
	AssetRenderPreviews         []AssetRenderPreview          `json:"asset_render_previews,omitempty"`
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews `json:"platform_asset_render_previews,omitempty"`
	AssetGenerationSummary      *AssetGenerationSummary       `json:"asset_generation_summary,omitempty"`
	AssetGenerationTasks        []assetgeneration.Task        `json:"asset_generation_tasks,omitempty"`
	AssetGenerationQueue        *GenerationWorkQueue          `json:"asset_generation_queue,omitempty"`
	AssetGenerationOverview     *AssetGenerationOverview      `json:"asset_generation_overview,omitempty"`
	Overview                    *ListingKitExportMeta         `json:"overview,omitempty"`
	Amazon                      *AmazonExportPayload          `json:"amazon,omitempty"`
	Shein                       *SheinExportPayload           `json:"shein,omitempty"`
	Temu                        *TemuExportPayload            `json:"temu,omitempty"`
	Walmart                     *WalmartExportPayload         `json:"walmart,omitempty"`
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
