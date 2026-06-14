package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewHeaderFromReadProjection(projection *listingKitReadProjection) *ListingKitPreviewHeader {
	if projection == nil || projection.PreviewInput.Overview == nil {
		return nil
	}
	return adaptPreviewDomainHeaderWithLegacyPlatformCards(
		previewdomain.BuildHeader(*projection.PreviewInput.Overview),
		projection.PlatformCards,
	)
}

func buildListingKitExportMetaFromReadProjection(projection *listingKitReadProjection) *ListingKitExportMeta {
	if projection == nil {
		return nil
	}
	return adaptPreviewInputToExportMeta(projection.PreviewInput.Overview, projection.PlatformCards)
}

func buildListingKitPreviewDomainProjectionFromReadProjection(projection *listingKitReadProjection) *previewdomain.Preview {
	if projection == nil {
		return nil
	}
	return previewdomain.BuildReadModel(projection.previewDomainReadModelInput())
}
