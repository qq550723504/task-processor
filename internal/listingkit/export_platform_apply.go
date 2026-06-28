package listingkit

import listingplatform "task-processor/internal/listing/platform"

func applyExportPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return listingplatform.BuildOne(listingplatform.Section{
		SelectedPlatform: selectedPlatform,
		Platform:         platform,
		Available:        available,
		Build:            build,
		UnavailableError: ErrPreviewPlatformUnavailable,
	})
}
