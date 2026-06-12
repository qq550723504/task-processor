package listingkit

type listingKitOverviewData struct {
	Country       string
	Language      string
	SourceType    string
	ImageCount    int
	VariantCount  int
	Warnings      []string
	ReviewReasons []string
	PlatformCards []ListingKitPlatformCard
}

func buildListingKitOverviewData(result *ListingKitResult, selectedPlatform string) *listingKitOverviewData {
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
	overview.ReviewReasons = reviewReasonsFromResult(result)
	overview.PlatformCards = buildPlatformPreviewCards(result, selectedPlatform)
	return overview
}
