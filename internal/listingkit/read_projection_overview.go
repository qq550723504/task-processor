package listingkit

func buildListingKitOverviewData(result *ListingKitResult, selectedPlatform string) *listingKitOverviewData {
	overview := initializeListingKitOverviewData(result)
	return decorateListingKitOverviewData(result, overview, selectedPlatform)
}
