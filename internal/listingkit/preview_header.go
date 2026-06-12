package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	if result == nil {
		return nil
	}

	headerInput := previewdomain.HeaderInput{
		Country:       result.Country,
		Language:      result.Language,
		StatusMessage: "预览结果已生成",
	}
	if result.Summary != nil {
		headerInput.SourceType = result.Summary.SourceType
		headerInput.ImageCount = result.Summary.ImageCount
		headerInput.VariantCount = result.Summary.VariantCount
		headerInput.Warnings = result.Summary.Warnings
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
	header.ReviewReasons = reviewReasonsFromResult(result)
	header.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return header
}
