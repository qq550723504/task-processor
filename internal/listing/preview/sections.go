package preview

import listingplatform "task-processor/internal/listing/platform"

func BuildPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return listingplatform.BuildOne(listingplatform.Section{
		SelectedPlatform: selectedPlatform,
		Platform:         platform,
		Available:        available,
		Build:            build,
		UnavailableError: ErrPlatformUnavailable,
	})
}

func PlatformUnavailableError(selectedPlatform, platform string) error {
	if listingplatform.IsSelected(selectedPlatform, platform) {
		return ErrPlatformUnavailable
	}
	return nil
}
