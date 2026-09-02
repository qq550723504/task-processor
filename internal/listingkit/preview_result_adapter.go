package listingkit

import (
	previewdomain "task-processor/internal/listing/preview"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type listingKitPreviewProjection struct {
	overview        *ListingKitPreviewHeader
	needsReview     bool
	attachment      listingKitPreviewProjectionAttachment
	revisionMeta    *ListingKitRevisionHistoryMeta
	revisionHistory []ListingKitRevisionRecord
}

type listingKitPreviewProjectionAttachment struct {
	catalog        *catalog.ProductSnapshot
	approvedAssets *productasset.ApprovedAssetInventory
}

func buildListingKitPreviewProjection(task *Task, selectedPlatform string) listingKitPreviewProjection {
	if task == nil || task.Result == nil {
		return listingKitPreviewProjection{}
	}
	readProjection := buildListingKitReadProjection(task.Result, selectedPlatform)
	if readProjection == nil {
		return listingKitPreviewProjection{}
	}
	base := buildListingKitTaskPreviewDomainProjection(task, readProjection, selectedPlatform)
	if base == nil {
		return listingKitPreviewProjection{}
	}
	domainProjection := buildPreviewDomainResultProjection(base)
	return adaptPreviewDomainResultProjection(domainProjection, readProjection, task.Result.RevisionHistory)
}

func buildPreviewDomainResultProjection(preview *previewdomain.Preview) previewdomain.ResultProjection {
	return previewdomain.BuildResultProjection(previewdomain.ResultProjectionInput{
		Preview: preview,
	})
}

func adaptPreviewDomainResultProjection(
	domainProjection previewdomain.ResultProjection,
	readProjection *listingKitReadProjection,
	revisionHistory []ListingKitRevisionRecord,
) listingKitPreviewProjection {
	projection := listingKitPreviewProjection{
		overview:    adaptPreviewDomainHeader(domainProjection.Overview),
		needsReview: domainProjection.NeedsReview,
		attachment: listingKitPreviewProjectionAttachment{
			catalog:        adaptPreviewDomainCatalog(domainProjection.Attachment),
			approvedAssets: adaptPreviewDomainApprovedAssets(domainProjection.Attachment),
		},
		revisionMeta:    adaptPreviewDomainRevisionHistoryMeta(domainProjection.RevisionHistoryMeta),
		revisionHistory: buildRevisionHistoryPreviewItems(revisionHistory),
	}
	if readProjection == nil {
		return projection
	}
	projection.overview = adaptPreviewDomainHeaderWithLegacyPlatformCards(domainProjection.Overview, readProjection.PlatformCards)
	return projection
}

func applyListingKitPreviewProjection(preview *ListingKitPreview, projection listingKitPreviewProjection) {
	if preview == nil {
		return
	}
	preview.Overview = projection.overview
	preview.NeedsReview = projection.needsReview
	preview.Catalog = projection.attachment.catalog
	preview.ApprovedAssetInventory = cloneApprovedAssetInventory(projection.attachment.approvedAssets)
	preview.RevisionHistoryMeta = projection.revisionMeta
	preview.RevisionHistory = projection.revisionHistory
}
