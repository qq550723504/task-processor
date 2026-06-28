package preview

import listingplatform "task-processor/internal/listing/platform"

func SupportedPlatforms() []string {
	return listingplatform.SupportedPlatforms()
}

func NormalizePlatform(platform string) string {
	return listingplatform.Normalize(platform)
}

func IsSupportedPlatform(platform string) bool {
	return listingplatform.IsSupported(platform)
}

func ValidateSelectedPlatform(platform string) (string, bool) {
	return listingplatform.ValidateSelectedPlatform(platform)
}

func ShouldBuildPlatform(selectedPlatform, platform string) bool {
	return listingplatform.ShouldBuild(selectedPlatform, platform)
}

func IsSelectedPlatform(selectedPlatform, platform string) bool {
	return listingplatform.IsSelected(selectedPlatform, platform)
}
