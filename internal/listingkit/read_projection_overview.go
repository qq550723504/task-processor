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
	overview := initializeListingKitOverviewData(result)
	return decorateListingKitOverviewData(result, overview, selectedPlatform)
}
