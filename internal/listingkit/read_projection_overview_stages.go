package listingkit

func initializeListingKitOverviewData(result *ListingKitResult) *listingKitOverviewData {
	if result == nil {
		return nil
	}

	overview := &listingKitOverviewData{
		Country:  result.Country,
		Language: result.Language,
	}
	if result.Summary != nil {
		overview.SourceType = result.Summary.SourceType
		overview.ImageCount = result.Summary.ImageCount
		overview.VariantCount = result.Summary.VariantCount
		overview.Warnings = append([]string(nil), result.Summary.Warnings...)
	}
	return overview
}

func decorateListingKitOverviewData(result *ListingKitResult, overview *listingKitOverviewData, selectedPlatform string) *listingKitOverviewData {
	if result == nil || overview == nil {
		return overview
	}
	overview.ReviewReasons = reviewReasonsFromResult(result)
	overview.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return overview
}
