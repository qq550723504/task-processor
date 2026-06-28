package consumer

import (
	"fmt"
	"strings"

	"task-processor/internal/core/config"
)

type PlatformModuleCatalog struct {
	selection platformSelection
	modules   []PlatformModule
}

func NewPlatformModuleCatalog(cfg *config.Config, platformsStr string, modules []PlatformModule) PlatformModuleCatalog {
	return PlatformModuleCatalog{
		selection: newPlatformSelection(cfg, platformsStr, modules),
		modules:   modules,
	}
}

func (c PlatformModuleCatalog) EnabledPlatformNames() []string {
	return c.selection.names()
}

func (c PlatformModuleCatalog) Resolve(platform string) (PlatformModule, error) {
	return c.resolveModule(platform)
}

func (c PlatformModuleCatalog) ResolveMany(platforms ...string) ([]PlatformModule, error) {
	if len(platforms) == 0 {
		return c.selection.enabledModules(c.modules), nil
	}

	modules := make([]PlatformModule, 0, len(platforms))
	seen := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		normalized := strings.ToLower(strings.TrimSpace(platform))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		module, err := c.resolveModule(normalized)
		if err != nil {
			return nil, err
		}
		modules = append(modules, module)
		seen[normalized] = struct{}{}
	}
	return modules, nil
}

func (c PlatformModuleCatalog) IsEnabled(platform string) bool {
	return c.selection.isEnabled(platform)
}

func (c PlatformModuleCatalog) resolveModule(platform string) (PlatformModule, error) {
	module, ok := c.find(platform)
	if !ok {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
	if !c.selection.isEnabled(platform) {
		return nil, fmt.Errorf("%s platform is not enabled", strings.ToUpper(platform))
	}
	return module, nil
}

func (c PlatformModuleCatalog) find(platform string) (PlatformModule, bool) {
	for _, module := range c.modules {
		if strings.EqualFold(module.Name(), platform) {
			return module, true
		}
	}
	return nil, false
}

func platformModuleNames(modules []PlatformModule) []string {
	names := make([]string, 0, len(modules))
	for _, module := range modules {
		names = append(names, module.Name())
	}
	return names
}
