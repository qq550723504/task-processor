package preview

type PlatformSectionBuilder struct {
	Platform string
	Build    func() error
}

func BuildPlatformSections(builders []PlatformSectionBuilder) error {
	for _, builder := range builders {
		if builder.Build == nil {
			continue
		}
		if err := builder.Build(); err != nil {
			return err
		}
	}
	return nil
}

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
