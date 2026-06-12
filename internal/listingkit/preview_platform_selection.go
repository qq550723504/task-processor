package listingkit

import "strings"

func validateSelectedPreviewPlatform(selectedPlatform string) (string, error) {
	selectedPlatform = normalizePreviewPlatform(selectedPlatform)
	if selectedPlatform != "" && len(normalizePlatforms([]string{selectedPlatform})) == 0 {
		return "", ErrUnsupportedPreviewPlatform
	}
	return selectedPlatform, nil
}

func normalizePreviewPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func shouldBuildPreviewPlatform(selectedPlatform, platform string) bool {
	return selectedPlatform == "" || isSelectedPreviewPlatform(selectedPlatform, platform)
}

func isSelectedPreviewPlatform(selectedPlatform, platform string) bool {
	return selectedPlatform == platform
}
