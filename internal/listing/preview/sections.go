package preview

func BuildPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	if !ShouldBuildPlatform(selectedPlatform, platform) {
		return nil
	}
	if !available {
		return PlatformUnavailableError(selectedPlatform, platform)
	}
	build()
	return nil
}

func PlatformUnavailableError(selectedPlatform, platform string) error {
	if IsSelectedPlatform(selectedPlatform, platform) {
		return ErrPlatformUnavailable
	}
	return nil
}
