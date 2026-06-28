package consumer

import (
	"strings"

	"task-processor/internal/core/config"
)

type platformSelection struct {
	enabled []string
}

func newPlatformSelection(cfg *config.Config, platformsStr string, modules []PlatformModule) platformSelection {
	if platformsStr != "" {
		return platformSelection{enabled: parsePlatformList(platformsStr)}
	}
	return platformSelection{enabled: enabledPlatformsFromModules(cfg, modules)}
}

func (s platformSelection) names() []string {
	return append([]string(nil), s.enabled...)
}

func (s platformSelection) isEnabled(platform string) bool {
	return containsPlatform(s.enabled, platform)
}

func (s platformSelection) enabledModules(modules []PlatformModule) []PlatformModule {
	enabled := make([]PlatformModule, 0, len(modules))
	for _, module := range modules {
		if s.isEnabled(module.Name()) {
			enabled = append(enabled, module)
		}
	}
	return enabled
}

func parsePlatformList(platformsStr string) []string {
	platforms := strings.Split(platformsStr, ",")
	result := make([]string, 0, len(platforms))

	for _, platform := range platforms {
		trimmed := strings.TrimSpace(strings.ToLower(platform))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

func enabledPlatformsFromModules(cfg *config.Config, modules []PlatformModule) []string {
	platforms := make([]string, 0)
	for _, module := range modules {
		if module.Enabled(cfg) {
			platforms = append(platforms, module.Name())
		}
	}

	return platforms
}

func containsPlatform(platforms []string, platform string) bool {
	platform = strings.ToLower(platform)
	for _, enabled := range platforms {
		if strings.ToLower(enabled) == platform {
			return true
		}
	}
	return false
}
