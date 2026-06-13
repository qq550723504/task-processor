package listingkit

import (
	previewdomain "task-processor/internal/listing/preview"
	legacylistingkit "task-processor/internal/listingkit"
)

// AdaptLegacyPreviewShell converts the legacy ListingKit preview shell into the
// new listing preview shell model without carrying platform-specific payloads.
func AdaptLegacyPreviewShell(legacy *legacylistingkit.ListingKitPreview) *previewdomain.Preview {
	if legacy == nil {
		return nil
	}
	return previewdomain.BuildProjection(previewdomain.ProjectionInput{
		Shell: previewdomain.ShellInput{
			TaskID:           legacy.TaskID,
			Status:           string(legacy.Status),
			SelectedPlatform: legacy.SelectedPlatform,
			Platforms:        legacy.Platforms,
			CreatedAt:        legacy.CreatedAt,
			CompletedAt:      legacy.CompletedAt,
		},
		NeedsReview:         legacy.NeedsReview,
		Attachment:          adaptLegacyPreviewAttachmentInput(legacy),
		Overview:            adaptLegacyPreviewHeaderInput(legacy.Overview),
		RevisionHistoryMeta: adaptLegacyRevisionHistoryMetaInput(legacy.RevisionHistoryMeta),
	})
}

func adaptLegacyPreviewAttachmentInput(legacy *legacylistingkit.ListingKitPreview) *previewdomain.AttachmentInput {
	if legacy == nil {
		return nil
	}
	return &previewdomain.AttachmentInput{
		CatalogProduct:        legacy.Catalog,
		AssetBundle:           legacy.Assets,
		AssetInventorySummary: legacy.AssetInventory,
	}
}

func adaptLegacyPreviewHeaderInput(legacy *legacylistingkit.ListingKitPreviewHeader) *previewdomain.HeaderInput {
	if legacy == nil {
		return nil
	}
	input := &previewdomain.HeaderInput{
		Country:       legacy.Country,
		Language:      legacy.Language,
		SourceType:    legacy.SourceType,
		ImageCount:    legacy.ImageCount,
		VariantCount:  legacy.VariantCount,
		StatusMessage: legacy.StatusMessage,
		Warnings:      legacy.Warnings,
		ReviewReasons: legacy.ReviewReasons,
	}
	if len(legacy.PlatformCards) > 0 {
		input.PlatformCards = make([]previewdomain.PlatformCard, 0, len(legacy.PlatformCards))
		for _, card := range legacy.PlatformCards {
			input.PlatformCards = append(input.PlatformCards, previewdomain.PlatformCard{
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
				Warnings:              append([]string(nil), legacy.Warnings...),
			})
		}
	}
	return input
}

func adaptLegacyRevisionHistoryMetaInput(legacy *legacylistingkit.ListingKitRevisionHistoryMeta) *previewdomain.RevisionHistoryMetaInput {
	if legacy == nil {
		return nil
	}
	return &previewdomain.RevisionHistoryMetaInput{
		TotalRecords:    legacy.TotalRecords,
		ReturnedRecords: legacy.ReturnedRecords,
		MaxRecords:      legacy.MaxRecords,
	}
}
