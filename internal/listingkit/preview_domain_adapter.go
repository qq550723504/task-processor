package listingkit

import previewdomain "task-processor/internal/listing/preview"

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
	return &ListingKitPreviewHeader{
		Country:       base.Country,
		Language:      base.Language,
		SourceType:    base.SourceType,
		ImageCount:    base.ImageCount,
		VariantCount:  base.VariantCount,
		StatusMessage: base.StatusMessage,
		Warnings:      append([]string(nil), base.Warnings...),
		ReviewReasons: append([]string(nil), base.ReviewReasons...),
	}
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
