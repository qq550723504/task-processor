package listingkit

import "task-processor/internal/asset"

func buildTemuPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "temu"
	return buildPreviewPlatformSection(selectedPlatform, platform, task.Result.Temu != nil, func() {
		preview.Temu = buildTemuPreviewPayload(
			task.Result.Temu,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		preview.NeedsReview = preview.NeedsReview || preview.Temu.NeedsReview
	})
}

func buildTemuPreviewPayload(pkg *TemuPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *TemuPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &TemuPreviewPayload{
		Headline:       pkg.GoodsName,
		NeedsReview:    len(pkg.ReviewNotes) > 0,
		ReviewNotes:    append([]string(nil), pkg.ReviewNotes...),
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Package:        pkg,
	}
}
