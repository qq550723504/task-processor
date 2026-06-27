package listingkit

import previewdomain "task-processor/internal/listing/preview"

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
	domainProjection := previewdomain.BuildResultProjection(previewdomain.ResultProjectionInput{
		Preview: base,
	})
	return adaptPreviewDomainResultProjection(domainProjection, readProjection, task.Result.RevisionHistory)
}

func applyListingKitPreviewProjection(preview *ListingKitPreview, projection listingKitPreviewProjection) {
	if preview == nil {
		return
	}
	preview.Overview = projection.overview
	preview.NeedsReview = projection.needsReview
	preview.Catalog = projection.attachment.catalog
	preview.Assets = projection.attachment.assets
	preview.AssetInventory = projection.attachment.assetInventory
	preview.AssetRenderPreviews = projection.attachment.assetRenderPreviews
	preview.PlatformAssetRenderPreviews = projection.attachment.platformPreviews
	preview.AssetGenerationQueue = projection.attachment.generationQueue
	preview.AssetGenerationOverview = projection.attachment.generationOverview
	preview.RevisionHistoryMeta = projection.revisionMeta
	preview.RevisionHistory = projection.revisionHistory
}

func buildListingKitTaskPreviewDomainProjection(
	task *Task,
	readProjection *listingKitReadProjection,
	selectedPlatform string,
) *previewdomain.Preview {
	if task == nil || task.Result == nil || readProjection == nil {
		return nil
	}
	return previewdomain.BuildTaskReadModel(previewdomain.TaskReadModelInput{
		Task: previewdomain.TaskShellInput{
			TaskID:           task.ID,
			Status:           string(task.Status),
			SelectedPlatform: selectedPlatform,
			ResultPlatforms:  task.Result.Platforms,
			RequestPlatforms: previewRequestPlatforms(task),
			CreatedAt:        task.CreatedAt,
			UpdatedAt:        task.UpdatedAt,
		},
		ReadModel: readProjection.previewDomainReadModelInput(),
	})
}

func previewRequestPlatforms(task *Task) []string {
	if task == nil || task.Request == nil {
		return nil
	}
	return task.Request.Platforms
}
