package listingkit

func applyExportPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return applyPlatformSection(selectedPlatform, platform, available, build)
}
