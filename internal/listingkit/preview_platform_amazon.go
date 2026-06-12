package listingkit

import "task-processor/internal/asset"

func buildAmazonPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "amazon"
	return applyPreviewPlatformSection(selectedPlatform, platform, task.Result.Amazon != nil, func() {
		preview.Amazon = buildAmazonPreviewPayload(
			task.Result.Amazon,
			task.Result.AssetBundle,
			platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
		)
	})
}

func buildAmazonPreviewPayload(pkg *AmazonPackage, assetBundle *asset.Bundle, renderPreviews *PlatformAssetRenderPreviews) *AmazonPreviewPayload {
	if pkg == nil || pkg.Draft == nil {
		return nil
	}
	base := buildPlatformVisualPreviewBase(pkg.ImageBundle, assetBundle, renderPreviews)
	return &AmazonPreviewPayload{
		Title:          pkg.Draft.Title,
		Brand:          pkg.Draft.Brand,
		ProductType:    pkg.Draft.ProductType,
		ImageBundle:    base.imageBundle,
		RenderPreviews: base.renderPreviews,
		ScenePresets:   base.scenePresets,
		Draft:          pkg.Draft,
	}
}
