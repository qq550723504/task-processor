package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	overview := buildListingKitOverviewData(result, selectedPlatform)
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
	header := &ListingKitPreviewHeader{
		Country:       baseHeader.Country,
		Language:      baseHeader.Language,
		SourceType:    baseHeader.SourceType,
		ImageCount:    baseHeader.ImageCount,
		VariantCount:  baseHeader.VariantCount,
		StatusMessage: baseHeader.StatusMessage,
		Warnings:      append([]string(nil), baseHeader.Warnings...),
	}
	header.ReviewReasons = append([]string(nil), overview.ReviewReasons...)
	header.PlatformCards = append([]ListingKitPlatformCard(nil), overview.PlatformCards...)
	return header
}
