package httpapi

import (
	"net/http"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

func buildHTTPServerBundleFromModules(port int, cfg *config.Config, modules []kernelmodule.Module) (*http.Server, []routeDescriptor, error) {
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, nil, err
	}
	server, routes := bundle.buildServerBundle(port)
	return server, routes, nil
}

func buildRegisteredRoutesForModules(cfg *config.Config, modules []kernelmodule.Module) ([]routeDescriptor, error) {
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, err
	}
	return bundle.routes, nil
}
