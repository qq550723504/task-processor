package listingkit

import previewdomain "task-processor/internal/listing/preview"

func applyExportPlatformSection(selectedPlatform, platform string, available bool, build func()) error {
	return adaptPreviewPlatformSectionError(previewdomain.BuildPlatformSection(selectedPlatform, platform, available, build))
}
