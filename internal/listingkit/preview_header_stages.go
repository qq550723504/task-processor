package listingkit

import previewdomain "task-processor/internal/listing/preview"

func initializePreviewHeader(overview *listingKitOverviewData) *ListingKitPreviewHeader {
	if overview == nil {
		return nil
	}

	baseHeader := previewdomain.BuildHeader(*buildPreviewDomainHeaderInput(overview))
	return &ListingKitPreviewHeader{
		Country:       baseHeader.Country,
		Language:      baseHeader.Language,
		SourceType:    baseHeader.SourceType,
		ImageCount:    baseHeader.ImageCount,
		VariantCount:  baseHeader.VariantCount,
		StatusMessage: baseHeader.StatusMessage,
		Warnings:      append([]string(nil), baseHeader.Warnings...),
		ReviewReasons: append([]string(nil), baseHeader.ReviewReasons...),
	}
}

func decoratePreviewHeader(overview *listingKitOverviewData, header *ListingKitPreviewHeader) *ListingKitPreviewHeader {
	if overview == nil || header == nil {
		return header
	}
	header.PlatformCards = append([]ListingKitPlatformCard(nil), overview.PlatformCards...)
	return header
}
