package listingkit

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
