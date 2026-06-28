package listingkit

import "task-processor/internal/listing/platformsection"

func applyExportPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return platformsection.BuildOne(platformsection.Section{
		SelectedPlatform: selectedPlatform,
		Platform:         platform,
		Available:        available,
		Build:            build,
		UnavailableError: ErrPreviewPlatformUnavailable,
	})
}
