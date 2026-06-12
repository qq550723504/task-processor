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
	if result == nil {
		return nil
	}

	attachment := &listingKitResultAttachment{
		CatalogProduct:        result.CatalogProduct,
		AssetBundle:           result.AssetBundle,
		AssetInventorySummary: result.AssetInventorySummary,
		AssetRenderPreviews:   append([]AssetRenderPreview(nil), result.AssetRenderPreviews...),
		PlatformAssetRenderPreviews: append(
			[]PlatformAssetRenderPreviews(nil),
			result.PlatformAssetRenderPreviews...,
		),
		AssetGenerationQueue:    result.AssetGenerationQueue,
		AssetGenerationOverview: result.AssetGenerationOverview,
	}
	if len(attachment.AssetRenderPreviews) == 0 {
		attachment.AssetRenderPreviews = buildAssetRenderPreviews(result.AssetBundle)
	}
	if len(attachment.PlatformAssetRenderPreviews) == 0 {
		attachment.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(result)
	}
	attachment.PlatformAssetRenderPreviews = filterPlatformAssetRenderPreviews(attachment.PlatformAssetRenderPreviews, selectedPlatform)
	return attachment
}
