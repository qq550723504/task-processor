package httpapi

import (
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

func buildRegisteredRoutesForModules(cfg *config.Config, modules []kernelmodule.Module) ([]routeDescriptor, error) {
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, err
	}
	return bundle.routes, nil
}
