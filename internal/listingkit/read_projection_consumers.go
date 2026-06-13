package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewHeaderFromReadProjection(projection *listingKitReadProjection) *ListingKitPreviewHeader {
	if projection == nil {
		return nil
	}
	return buildPreviewHeaderFromOverview(projection.Overview)
}

func buildListingKitExportMetaFromReadProjection(projection *listingKitReadProjection) *ListingKitExportMeta {
	if projection == nil {
		return nil
	}
	return buildListingKitExportMetaFromOverview(projection.Overview)
}

func buildListingKitPreviewDomainProjectionFromReadProjection(
	result *ListingKitResult,
	projection *listingKitReadProjection,
) *previewdomain.Preview {
	if projection == nil {
		return nil
	}
	return previewdomain.BuildProjection(previewdomain.ProjectionInput{
		NeedsReview:         projection.NeedsReview,
		Attachment:          buildPreviewDomainAttachmentInput(projection.Attachment),
		Overview:            buildPreviewDomainHeaderInput(projection.Overview),
		RevisionHistoryMeta: buildPreviewDomainRevisionHistoryMetaInput(result),
	})
}
