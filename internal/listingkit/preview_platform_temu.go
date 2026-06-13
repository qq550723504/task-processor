package listingkit

import "task-processor/internal/asset"

func buildTemuPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "temu"
	return applyReviewablePreviewPlatformSection(selectedPlatform, platform, task.Result.Temu != nil, preview, func() bool {
		preview.Temu = buildTemuPreviewPayload(
			task.Result.Temu,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		return preview.Temu != nil && preview.Temu.NeedsReview
	})
}

func buildTemuPreviewPayload(pkg *TemuPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	base := buildReviewablePlatformPreviewBase(pkg.ReviewNotes, pkg.ImageBundle, assetBundle, renderPreviews)
	return buildTemuPreviewPayloadBody(reviewablePlatformPreviewPayloadInput{
		base: buildReviewablePlatformPreviewPayloadBase(pkg.GoodsName, base),
	}, pkg)
}
