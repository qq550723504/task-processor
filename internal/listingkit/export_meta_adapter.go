package listingkit

import previewdomain "task-processor/internal/listing/preview"

func adaptPreviewInputToExportMeta(overview *previewdomain.HeaderInput, platformCards []ListingKitPlatformCard) *ListingKitExportMeta {
	if overview == nil {
		return nil
	}
	return &ListingKitExportMeta{
		Country:       overview.Country,
		Language:      overview.Language,
		SourceType:    overview.SourceType,
		ImageCount:    overview.ImageCount,
		VariantCount:  overview.VariantCount,
		Warnings:      append([]string(nil), overview.Warnings...),
		ReviewReasons: append([]string(nil), overview.ReviewReasons...),
		PlatformCards: append([]ListingKitPlatformCard(nil), platformCards...),
	}
}
