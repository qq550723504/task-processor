package listingkit

import previewdomain "task-processor/internal/listing/preview"

func validateSelectedPreviewPlatform(selectedPlatform string) (string, error) {
	selectedPlatform, ok := previewdomain.ValidateSelectedPlatform(selectedPlatform)
	if !ok {
		return "", ErrUnsupportedPreviewPlatform
	}
	return selectedPlatform, nil
}

func normalizePreviewPlatform(platform string) string {
	return previewdomain.NormalizePlatform(platform)
}

func shouldBuildPreviewPlatform(selectedPlatform, platform string) bool {
	return previewdomain.ShouldBuildPlatform(selectedPlatform, platform)
}

func isSelectedPreviewPlatform(selectedPlatform, platform string) bool {
	return previewdomain.IsSelectedPlatform(selectedPlatform, platform)
}
