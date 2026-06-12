package listingkit

import previewdomain "task-processor/internal/listing/preview"

func initializePreviewHeader(overview *listingKitOverviewData) *ListingKitPreviewHeader {
	if overview == nil {
		return nil
	}

	headerInput := previewdomain.HeaderInput{
		Country:       overview.Country,
		Language:      overview.Language,
		StatusMessage: "预览结果已生成",
		SourceType:    overview.SourceType,
		ImageCount:    overview.ImageCount,
		VariantCount:  overview.VariantCount,
		Warnings:      overview.Warnings,
	}
	baseHeader := previewdomain.BuildHeader(headerInput)
	return &ListingKitPreviewHeader{
		Country:       baseHeader.Country,
		Language:      baseHeader.Language,
		SourceType:    baseHeader.SourceType,
		ImageCount:    baseHeader.ImageCount,
		VariantCount:  baseHeader.VariantCount,
		StatusMessage: baseHeader.StatusMessage,
		Warnings:      append([]string(nil), baseHeader.Warnings...),
	}
}

func decoratePreviewHeader(overview *listingKitOverviewData, header *ListingKitPreviewHeader) *ListingKitPreviewHeader {
	if overview == nil || header == nil {
		return header
	}
	header.ReviewReasons = append([]string(nil), overview.ReviewReasons...)
	header.PlatformCards = append([]ListingKitPlatformCard(nil), overview.PlatformCards...)
	return header
}
