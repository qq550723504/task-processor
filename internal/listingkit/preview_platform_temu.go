package listingkit

import "task-processor/internal/asset"

func buildTemuPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "temu"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, task.Result != nil && task.Result.Temu != nil, preview, func() bool {
		preview.Temu = buildTemuPreviewPayloadFromResult(task.Result, preview.PlatformAssetRenderPreviews)
		return preview.Temu != nil && preview.Temu.NeedsReview
	})
}

func buildTemuPreviewPayload(pkg *TemuPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return buildTemuPreviewPayloadBody(
		buildReviewablePlatformPreviewPayloadInput(
			pkg.GoodsName,
			pkg.ReviewNotes,
			pkg.ImageBundle,
			assetBundle,
			renderPreviews,
		),
		pkg,
	)
}
