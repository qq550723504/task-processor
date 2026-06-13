package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewHeader(result *ListingKitResult, selectedPlatform string) *ListingKitPreviewHeader {
	projection := buildListingKitReadProjection(result, selectedPlatform)
	if projection == nil {
		return nil
	}
	return buildPreviewHeaderFromOverview(projection.Overview)
}

func buildPreviewHeaderFromOverview(overview *listingKitOverviewData) *ListingKitPreviewHeader {
	if overview == nil {
		return nil
	}
	return adaptPreviewDomainHeaderWithLegacyPlatformCards(
		previewdomain.BuildHeader(*buildPreviewDomainHeaderInput(overview)),
		overview,
	)
}
