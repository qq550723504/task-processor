package listingkit

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	if result == nil {
		return nil
	}

	header := &ListingKitPreviewHeader{
		Country:       result.Country,
		Language:      result.Language,
		StatusMessage: "预览结果已生成",
	}
	if result.Summary != nil {
		header.SourceType = result.Summary.SourceType
		header.ImageCount = result.Summary.ImageCount
		header.VariantCount = result.Summary.VariantCount
		header.Warnings = append([]string(nil), result.Summary.Warnings...)
	}
	header.ReviewReasons = reviewReasonsFromResult(result)
	header.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return header
}
