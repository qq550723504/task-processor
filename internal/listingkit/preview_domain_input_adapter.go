package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewDomainAttachmentInput(attachment *listingKitResultAttachment) *previewdomain.AttachmentInput {
	if attachment == nil {
		return nil
	}
	return &previewdomain.AttachmentInput{
		CatalogProduct:        attachment.CatalogProduct,
		AssetBundle:           attachment.AssetBundle,
		AssetInventorySummary: attachment.AssetInventorySummary,
	}
}

func buildPreviewDomainHeaderInput(overview *listingKitOverviewData) *previewdomain.HeaderInput {
	if overview == nil {
		return nil
	}
	input := &previewdomain.HeaderInput{
		Country:       overview.Country,
		Language:      overview.Language,
		StatusMessage: "预览结果已生成",
		SourceType:    overview.SourceType,
		ImageCount:    overview.ImageCount,
		VariantCount:  overview.VariantCount,
		Warnings:      overview.Warnings,
		ReviewReasons: overview.ReviewReasons,
	}
	if len(overview.PlatformCards) > 0 {
		input.PlatformCards = make([]previewdomain.PlatformCard, 0, len(overview.PlatformCards))
		for _, card := range overview.PlatformCards {
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
			})
		}
	}
	return input
}

func buildPreviewDomainRevisionHistoryMetaInput(result *ListingKitResult) *previewdomain.RevisionHistoryMetaInput {
	if result == nil {
		return nil
	}
	total := result.RevisionHistoryTotal
	if total == 0 && len(result.RevisionHistory) > 0 {
		total = len(result.RevisionHistory)
	}
	return &previewdomain.RevisionHistoryMetaInput{
		TotalRecords:    total,
		ReturnedRecords: len(result.RevisionHistory),
		MaxRecords:      maxRevisionHistoryRecords,
	}
}
