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
	base := buildListingKitPreviewDomainProjection(result, selectedPlatform)
	if base == nil {
		return listingKitPreviewProjection{}
	}
	readProjection := buildListingKitReadProjection(result, selectedPlatform)
	if readProjection == nil {
		return listingKitPreviewProjection{}
	}
	legacyBase := adaptPreviewDomainShell(base)
	if legacyBase.Overview != nil && readProjection.Overview != nil {
		legacyBase.Overview.PlatformCards = append([]ListingKitPlatformCard(nil), readProjection.Overview.PlatformCards...)
	}
	return listingKitPreviewProjection{
		overview:            legacyBase.Overview,
		needsReview:         legacyBase.NeedsReview,
		catalog:             readProjection.Attachment.CatalogProduct,
		assets:              readProjection.Attachment.AssetBundle,
		assetInventory:      readProjection.Attachment.AssetInventorySummary,
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
	if readProjection == nil {
		return nil
	}

	var headerInput *previewdomain.HeaderInput
	if readProjection.Overview != nil {
		headerInput = &previewdomain.HeaderInput{
			Country:       readProjection.Overview.Country,
			Language:      readProjection.Overview.Language,
			StatusMessage: "预览结果已生成",
			SourceType:    readProjection.Overview.SourceType,
			ImageCount:    readProjection.Overview.ImageCount,
			VariantCount:  readProjection.Overview.VariantCount,
			Warnings:      readProjection.Overview.Warnings,
			ReviewReasons: readProjection.Overview.ReviewReasons,
		}
		if len(readProjection.Overview.PlatformCards) > 0 {
			headerInput.PlatformCards = make([]previewdomain.PlatformCard, 0, len(readProjection.Overview.PlatformCards))
			for _, card := range readProjection.Overview.PlatformCards {
				headerInput.PlatformCards = append(headerInput.PlatformCards, previewdomain.PlatformCard{
					Platform:              card.Platform,
					Status:                card.Status,
					Summary:               card.Summary,
					NeedsReview:           card.NeedsReview,
					PreviewableItems:      card.PreviewableItems,
					ApprovedSections:      card.ApprovedSections,
					DeferredSections:      card.DeferredSections,
					ReviewPendingSections: card.ReviewPendingSections,
					PrimaryActionKey:      card.PrimaryActionKey,
					PrimaryCTAKind:        card.PrimaryCTAKind,
				})
			}
		}
	}

	var revisionMetaInput *previewdomain.RevisionHistoryMetaInput
	if result != nil {
		total := result.RevisionHistoryTotal
		if total == 0 && len(result.RevisionHistory) > 0 {
			total = len(result.RevisionHistory)
		}
		revisionMetaInput = &previewdomain.RevisionHistoryMetaInput{
			TotalRecords:    total,
			ReturnedRecords: len(result.RevisionHistory),
			MaxRecords:      maxRevisionHistoryRecords,
		}
	}

	return previewdomain.BuildProjection(previewdomain.ProjectionInput{
		NeedsReview:         readProjection.NeedsReview,
		Overview:            headerInput,
		RevisionHistoryMeta: revisionMetaInput,
	})
}
