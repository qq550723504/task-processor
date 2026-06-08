package listingkit

func buildAmazonPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "amazon"
	if !shouldBuildPreviewPlatform(selectedPlatform, platform) {
		return nil
	}
	if task.Result.Amazon == nil {
		if isSelectedPreviewPlatform(selectedPlatform, platform) {
			return ErrPreviewPlatformUnavailable
		}
		return nil
	}
	preview.Amazon = buildAmazonPreviewPayload(
		task.Result.Amazon,
		task.Result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
	)
	return nil
}

func buildSheinPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "shein"
	if !shouldBuildPreviewPlatform(selectedPlatform, platform) {
		return nil
	}
	if task.Result.Shein == nil {
		if isSelectedPreviewPlatform(selectedPlatform, platform) {
			return ErrPreviewPlatformUnavailable
		}
		return nil
	}
	preview.Shein = buildSheinPreviewPayload(
		task.Result.Shein,
		task.Result.PodExecution,
		task.Result.CanonicalProduct,
		task.Result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
	)
	preview.NeedsReview = preview.NeedsReview || preview.Shein.NeedsReview
	return nil
}

func buildTemuPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "temu"
	if !shouldBuildPreviewPlatform(selectedPlatform, platform) {
		return nil
	}
	if task.Result.Temu == nil {
		if isSelectedPreviewPlatform(selectedPlatform, platform) {
			return ErrPreviewPlatformUnavailable
		}
		return nil
	}
	preview.Temu = buildTemuPreviewPayload(
		task.Result.Temu,
		task.Result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
	)
	preview.NeedsReview = preview.NeedsReview || preview.Temu.NeedsReview
	return nil
}

func buildWalmartPreviewSection(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	const platform = "walmart"
	if !shouldBuildPreviewPlatform(selectedPlatform, platform) {
		return nil
	}
	if task.Result.Walmart == nil {
		if isSelectedPreviewPlatform(selectedPlatform, platform) {
			return ErrPreviewPlatformUnavailable
		}
		return nil
	}
	preview.Walmart = buildWalmartPreviewPayload(
		task.Result.Walmart,
		task.Result.AssetBundle,
		platformAssetRenderPreviewsByPlatform(preview.PlatformAssetRenderPreviews, platform),
	)
	preview.NeedsReview = preview.NeedsReview || preview.Walmart.NeedsReview
	return nil
}
