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

func buildPreviewPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	if !shouldBuildPreviewPlatform(selectedPlatform, platform) {
		return nil
	}
	if !available {
		return previewPlatformUnavailableError(selectedPlatform, platform)
	}
	build()
	return nil
}

func previewPlatformUnavailableError(selectedPlatform, platform string) error {
	if isSelectedPreviewPlatform(selectedPlatform, platform) {
		return ErrPreviewPlatformUnavailable
	}
	return nil
}
