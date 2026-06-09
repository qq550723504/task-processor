package listingkit

import "task-processor/internal/asset"

func buildWalmartPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "walmart"
	return buildPreviewPlatformSection(selectedPlatform, platform, task.Result.Walmart != nil, func() {
		preview.Walmart = buildWalmartPreviewPayload(
			task.Result.Walmart,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
		preview.NeedsReview = preview.NeedsReview || preview.Walmart.NeedsReview
	})
}

func buildWalmartPreviewPayload(pkg *WalmartPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *WalmartPreviewPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartPreviewPayload{
		Headline:       pkg.ProductName,
		NeedsReview:    len(pkg.ReviewNotes) > 0,
		ReviewNotes:    append([]string(nil), pkg.ReviewNotes...),
		ImageBundle:    pkg.ImageBundle,
		RenderPreviews: renderPreviews,
		ScenePresets:   buildPlatformScenePresetSummaries(pkg.ImageBundle, assetBundle),
		Package:        pkg,
	}
}
