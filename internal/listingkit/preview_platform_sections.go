package listingkit

import previewdomain "task-processor/internal/listing/preview"

func buildPreviewPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	err := previewdomain.BuildPlatformSection(selectedPlatform, platform, available, build)
	if err == previewdomain.ErrPlatformUnavailable {
		return ErrPreviewPlatformUnavailable
	}
	return err
}

func previewPlatformUnavailableError(selectedPlatform, platform string) error {
	err := previewdomain.PlatformUnavailableError(selectedPlatform, platform)
	if err == previewdomain.ErrPlatformUnavailable {
		return ErrPreviewPlatformUnavailable
	}
	return err
}
