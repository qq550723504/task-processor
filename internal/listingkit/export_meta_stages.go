package listingkit

func initializeListingKitExportMeta(overview *listingKitOverviewData) *ListingKitExportMeta {
	if overview == nil {
		return nil
	}
	return &ListingKitExportMeta{
		Country:      overview.Country,
		Language:     overview.Language,
		SourceType:   overview.SourceType,
		ImageCount:   overview.ImageCount,
		VariantCount: overview.VariantCount,
		Warnings:     append([]string(nil), overview.Warnings...),
	}
}

func decorateListingKitExportMeta(overview *listingKitOverviewData, meta *ListingKitExportMeta) *ListingKitExportMeta {
	if overview == nil || meta == nil {
		return meta
	}
	meta.ReviewReasons = append([]string(nil), overview.ReviewReasons...)
	meta.PlatformCards = append([]ListingKitPlatformCard(nil), overview.PlatformCards...)
	return meta
}
