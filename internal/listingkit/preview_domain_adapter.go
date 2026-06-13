package listingkit

import (
	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	previewdomain "task-processor/internal/listing/preview"
)

func adaptPreviewDomainShell(base *previewdomain.Preview) *ListingKitPreview {
	if base == nil {
		return nil
	}
	return &ListingKitPreview{
		TaskID:              base.TaskID,
		Status:              TaskStatus(base.Status),
		SelectedPlatform:    base.SelectedPlatform,
		Platforms:           append([]string(nil), base.Platforms...),
		NeedsReview:         base.NeedsReview,
		Catalog:             adaptPreviewDomainCatalog(base.Attachment),
		Assets:              adaptPreviewDomainAssets(base.Attachment),
		AssetInventory:      adaptPreviewDomainAssetInventory(base.Attachment),
		CreatedAt:           base.CreatedAt,
		CompletedAt:         base.CompletedAt,
		Overview:            adaptPreviewDomainHeader(base.Overview),
		RevisionHistoryMeta: adaptPreviewDomainRevisionHistoryMeta(base.RevisionHistoryMeta),
	}
}

func adaptPreviewDomainHeader(base *previewdomain.Header) *ListingKitPreviewHeader {
	if base == nil {
		return nil
	}
	header := &ListingKitPreviewHeader{
		Country:       base.Country,
		Language:      base.Language,
		SourceType:    base.SourceType,
		ImageCount:    base.ImageCount,
		VariantCount:  base.VariantCount,
		StatusMessage: base.StatusMessage,
		Warnings:      append([]string(nil), base.Warnings...),
		ReviewReasons: append([]string(nil), base.ReviewReasons...),
	}
	if len(base.PlatformCards) > 0 {
		header.PlatformCards = make([]ListingKitPlatformCard, 0, len(base.PlatformCards))
		for _, card := range base.PlatformCards {
			header.PlatformCards = append(header.PlatformCards, ListingKitPlatformCard{
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
	return header
}

func adaptPreviewDomainCatalog(base *previewdomain.Attachment) *catalog.Product {
	if base == nil {
		return nil
	}
	return base.CatalogProduct
}

func adaptPreviewDomainAssets(base *previewdomain.Attachment) *asset.Bundle {
	if base == nil {
		return nil
	}
	return base.AssetBundle
}

func adaptPreviewDomainAssetInventory(base *previewdomain.Attachment) *asset.InventorySummary {
	if base == nil {
		return nil
	}
	return base.AssetInventorySummary
}

func adaptPreviewDomainRevisionHistoryMeta(base *previewdomain.RevisionHistoryMeta) *ListingKitRevisionHistoryMeta {
	if base == nil {
		return nil
	}
	return &ListingKitRevisionHistoryMeta{
		TotalRecords:    base.TotalRecords,
		ReturnedRecords: base.ReturnedRecords,
		HasMore:         base.HasMore,
		IsTruncated:     base.IsTruncated,
		MaxRecords:      base.MaxRecords,
	}
}
