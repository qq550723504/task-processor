package preview

import "strings"

var supportedPlatforms = []string{"amazon", "shein", "temu", "walmart"}

func SupportedPlatforms() []string {
	return append([]string(nil), supportedPlatforms...)
}

func NormalizePlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func IsSupportedPlatform(platform string) bool {
	platform = NormalizePlatform(platform)
	for _, supported := range supportedPlatforms {
		if supported == platform {
			return true
		}
	}
	return false
}

func ValidateSelectedPlatform(platform string) (string, bool) {
	platform = NormalizePlatform(platform)
	if platform == "" {
		return "", true
	}
	return platform, IsSupportedPlatform(platform)
}

func ShouldBuildPlatform(selectedPlatform, platform string) bool {
	selectedPlatform = NormalizePlatform(selectedPlatform)
	platform = NormalizePlatform(platform)
	return selectedPlatform == "" || selectedPlatform == platform
}

func IsSelectedPlatform(selectedPlatform, platform string) bool {
	return NormalizePlatform(selectedPlatform) == NormalizePlatform(platform)
}
