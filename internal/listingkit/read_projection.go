package listingkit

func buildListingKitReadProjection(result *ListingKitResult, selectedPlatform string) *listingKitReadProjection {
	if result == nil {
		return nil
	}

	return assembleListingKitReadProjection(
		calculateListingKitNeedsReview(result),
		buildListingKitOverviewData(result, selectedPlatform),
		buildListingKitResultAttachment(result, selectedPlatform),
	)
}
