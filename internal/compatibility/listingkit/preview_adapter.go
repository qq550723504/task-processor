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
	adapted := previewdomain.BuildShell(previewdomain.ShellInput{
		TaskID:           legacy.TaskID,
		Status:           string(legacy.Status),
		SelectedPlatform: legacy.SelectedPlatform,
		Platforms:        legacy.Platforms,
		CreatedAt:        legacy.CreatedAt,
		CompletedAt:      legacy.CompletedAt,
	})
	adapted.NeedsReview = legacy.NeedsReview
	adapted.Overview = adaptLegacyPreviewHeader(legacy.Overview)
	return adapted
}

func adaptLegacyPreviewHeader(legacy *legacylistingkit.ListingKitPreviewHeader) *previewdomain.Header {
	if legacy == nil {
		return nil
	}
	header := previewdomain.BuildHeader(previewdomain.HeaderInput{
		Country:       legacy.Country,
		Language:      legacy.Language,
		SourceType:    legacy.SourceType,
		ImageCount:    legacy.ImageCount,
		VariantCount:  legacy.VariantCount,
		StatusMessage: legacy.StatusMessage,
		Warnings:      legacy.Warnings,
	})
	header.ReviewReasons = append([]string(nil), legacy.ReviewReasons...)
	if len(legacy.PlatformCards) > 0 {
		header.PlatformCards = make([]previewdomain.PlatformCard, 0, len(legacy.PlatformCards))
		for _, card := range legacy.PlatformCards {
			header.PlatformCards = append(header.PlatformCards, previewdomain.PlatformCard{
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
	return header
}
