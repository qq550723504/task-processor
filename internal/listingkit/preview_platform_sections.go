package listingkit

type previewPlatformBuilder interface {
	platform() string
	build(task *Task, preview *ListingKitPreview, selectedPlatform string) error
}

type previewPlatformBuilderFunc struct {
	name string
	fn   func(task *Task, preview *ListingKitPreview, selectedPlatform string) error
}

func (b previewPlatformBuilderFunc) platform() string {
	return b.name
}

func (b previewPlatformBuilderFunc) build(task *Task, preview *ListingKitPreview, selectedPlatform string) error {
	return b.fn(task, preview, selectedPlatform)
}

func previewPlatformBuilders() []previewPlatformBuilder {
	return []previewPlatformBuilder{
		previewPlatformBuilderFunc{name: "amazon", fn: buildAmazonPreviewSection},
		previewPlatformBuilderFunc{name: "shein", fn: buildSheinPreviewSection},
		previewPlatformBuilderFunc{name: "temu", fn: buildTemuPreviewSection},
		previewPlatformBuilderFunc{name: "walmart", fn: buildWalmartPreviewSection},
	}
}

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
