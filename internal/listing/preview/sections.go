package preview

import "task-processor/internal/listing/platformsection"

func BuildPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return platformsection.BuildOne(platformsection.Section{
		SelectedPlatform: selectedPlatform,
		Platform:         platform,
		Available:        available,
		Build:            build,
		UnavailableError: ErrPlatformUnavailable,
	})
}

func PlatformUnavailableError(selectedPlatform, platform string) error {
	if platformsection.IsSelected(selectedPlatform, platform) {
		return ErrPlatformUnavailable
	}
	return nil
}
