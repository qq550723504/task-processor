package platform

import "strings"

var supportedPlatforms = []string{"amazon", "shein", "temu", "walmart"}

type Builder struct {
	Platform string
	Build    func() error
}

type Section struct {
	SelectedPlatform string
	Platform         string
	Available        bool
	Build            func()
	UnavailableError error
}

func Normalize(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func SupportedPlatforms() []string {
	return append([]string(nil), supportedPlatforms...)
}

func IsSupported(platform string) bool {
	platform = Normalize(platform)
	for _, supported := range supportedPlatforms {
		if supported == platform {
			return true
		}
	}
	return false
}

func NormalizeSupportedPlatforms(platforms []string) []string {
	if len(platforms) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		normalized := Normalize(platform)
		if !IsSupported(normalized) {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func ValidateSelectedPlatform(platform string) (string, bool) {
	platform = Normalize(platform)
	if platform == "" {
		return "", true
	}
	return platform, IsSupported(platform)
}

func ShouldBuild(selectedPlatform, platform string) bool {
	selectedPlatform = Normalize(selectedPlatform)
	platform = Normalize(platform)
	return selectedPlatform == "" || selectedPlatform == platform
}

func IsSelected(selectedPlatform, platform string) bool {
	return Normalize(selectedPlatform) == Normalize(platform)
}

func BuildOne(section Section) error {
	if !ShouldBuild(section.SelectedPlatform, section.Platform) {
		return nil
	}
	if !section.Available {
		if IsSelected(section.SelectedPlatform, section.Platform) {
			return section.UnavailableError
		}
		return nil
	}
	if section.Build != nil {
		section.Build()
	}
	return nil
}

func BuildAll(builders []Builder) error {
	for _, builder := range builders {
		if builder.Build == nil {
			continue
		}
		if err := builder.Build(); err != nil {
			return err
		}
	}
	return nil
}
