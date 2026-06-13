package listingkit

func adaptOverviewToExportMeta(overview *listingKitOverviewData) *ListingKitExportMeta {
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
		PlatformCards: append([]ListingKitPlatformCard(nil), overview.PlatformCards...),
	}
}
