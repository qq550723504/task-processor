package preview

import "task-processor/internal/listing/platformsection"

func SupportedPlatforms() []string {
	return platformsection.SupportedPlatforms()
}

func NormalizePlatform(platform string) string {
	return platformsection.Normalize(platform)
}

func IsSupportedPlatform(platform string) bool {
	return platformsection.IsSupported(platform)
}

func ValidateSelectedPlatform(platform string) (string, bool) {
	return platformsection.ValidateSelectedPlatform(platform)
}

func ShouldBuildPlatform(selectedPlatform, platform string) bool {
	return platformsection.ShouldBuild(selectedPlatform, platform)
}

func IsSelectedPlatform(selectedPlatform, platform string) bool {
	return platformsection.IsSelected(selectedPlatform, platform)
}
