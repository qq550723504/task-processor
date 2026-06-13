package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

type listingKitReadProjection struct {
	NeedsReview bool
	Overview    *listingKitOverviewData
	Attachment  *listingKitResultAttachment
}

type listingKitOverviewData struct {
	Country       string
	Language      string
	SourceType    string
	ImageCount    int
	VariantCount  int
	Warnings      []string
	ReviewReasons []string
	PlatformCards []ListingKitPlatformCard
}

type listingKitResultAttachment struct {
	CatalogProduct              *catalog.Product
	AssetBundle                 *asset.Bundle
	AssetInventorySummary       *asset.InventorySummary
	AssetRenderPreviews         []AssetRenderPreview
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews
	AssetGenerationQueue        *GenerationWorkQueue
	AssetGenerationOverview     *AssetGenerationOverview
}
