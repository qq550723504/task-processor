package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

type listingKitResultAttachment struct {
	CatalogProduct              *catalog.Product
	AssetBundle                 *asset.Bundle
	AssetInventorySummary       *asset.InventorySummary
	AssetRenderPreviews         []AssetRenderPreview
	PlatformAssetRenderPreviews []PlatformAssetRenderPreviews
	AssetGenerationQueue        *GenerationWorkQueue
	AssetGenerationOverview     *AssetGenerationOverview
}

func buildListingKitResultAttachment(result *ListingKitResult, selectedPlatform string) *listingKitResultAttachment {
	attachment := initializeListingKitResultAttachment(result)
	attachment = backfillListingKitResultAttachment(result, attachment)
	return selectListingKitResultAttachmentPlatform(attachment, selectedPlatform)
}
