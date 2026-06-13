package listingkit

import previewdomain "task-processor/internal/listing/preview"

func adaptPreviewDomainHeaderWithLegacyPlatformCards(base *previewdomain.Header, overview *listingKitOverviewData) *ListingKitPreviewHeader {
	header := adaptPreviewDomainHeader(base)
	if overview == nil || header == nil {
		return header
	}
	header.PlatformCards = append([]ListingKitPlatformCard(nil), overview.PlatformCards...)
	return header
}
