package httpapi

import (
	"fmt"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

func buildRegisteredRoutesForModules(cfg *config.Config, modules []kernelmodule.Module) ([]routeDescriptor, error) {
	reg := kernelmodule.NewRegistry()

	for _, module := range modules {
		if module == nil {
			continue
		}
		if !module.Enabled(cfg) {
			continue
		}
		if err := module.Register(reg); err != nil {
			return nil, fmt.Errorf("register module %s: %w", module.Name(), err)
		}
	}

	return reg.Routes(), nil
}
