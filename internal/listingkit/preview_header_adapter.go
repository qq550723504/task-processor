package listingkit

import previewdomain "task-processor/internal/listing/preview"

func adaptPreviewDomainHeaderWithLegacyPlatformCards(base *previewdomain.Header, platformCards []ListingKitPlatformCard) *ListingKitPreviewHeader {
	header := adaptPreviewDomainHeader(base)
	if header == nil {
		return header
	}
	header.PlatformCards = append([]ListingKitPlatformCard(nil), platformCards...)
	return header
}
