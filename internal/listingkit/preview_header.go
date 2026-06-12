package listingkit

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	projection := buildListingKitReadProjection(result, selectedPlatform)
	if projection == nil {
		return nil
	}
	return buildPreviewHeaderFromOverview(projection.Overview)
}

func buildPreviewHeaderFromOverview(overview *listingKitOverviewData) *ListingKitPreviewHeader {
	header := initializePreviewHeader(overview)
	return decoratePreviewHeader(overview, header)
}
