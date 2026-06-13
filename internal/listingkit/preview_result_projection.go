package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	previewdomain "task-processor/internal/listing/preview"
)

type listingKitPreviewProjection struct {
	overview            *ListingKitPreviewHeader
	needsReview         bool
	catalog             *catalog.Product
	assets              *asset.Bundle
	assetInventory      *asset.InventorySummary
	assetRenderPreviews []AssetRenderPreview
	platformPreviews    []PlatformAssetRenderPreviews
	generationQueue     *GenerationWorkQueue
	generationOverview  *AssetGenerationOverview
	revisionMeta        *ListingKitRevisionHistoryMeta
	revisionHistory     []ListingKitRevisionRecord
}

func buildListingKitPreviewProjection(result *ListingKitResult, selectedPlatform string) listingKitPreviewProjection {
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	if readProjection == nil {
		return listingKitPreviewProjection{}
	}
	base := buildListingKitPreviewDomainProjectionFromReadProjection(result, readProjection)
	if base == nil {
		return listingKitPreviewProjection{}
	}
	legacyBase := adaptPreviewDomainShell(base)
	legacyBase.Overview = adaptPreviewDomainHeaderWithLegacyPlatformCards(base.Overview, readProjection.Overview)
	return listingKitPreviewProjection{
		overview:            legacyBase.Overview,
		needsReview:         legacyBase.NeedsReview,
		catalog:             legacyBase.Catalog,
		assets:              legacyBase.Assets,
		assetInventory:      legacyBase.AssetInventory,
		assetRenderPreviews: readProjection.Attachment.AssetRenderPreviews,
		platformPreviews:    readProjection.Attachment.PlatformAssetRenderPreviews,
		generationQueue:     readProjection.Attachment.AssetGenerationQueue,
		generationOverview:  readProjection.Attachment.AssetGenerationOverview,
		revisionMeta:        legacyBase.RevisionHistoryMeta,
		revisionHistory:     buildRevisionHistoryPreviewItems(result.RevisionHistory),
	}
}

func buildListingKitPreviewDomainProjection(result *ListingKitResult, selectedPlatform string) *previewdomain.Preview {
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	return buildListingKitPreviewDomainProjectionFromReadProjection(result, readProjection)
}
