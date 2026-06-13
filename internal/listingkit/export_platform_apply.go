package listingkit

func applyExportPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return buildPreviewPlatformSection(selectedPlatform, platform, available, build)
}
