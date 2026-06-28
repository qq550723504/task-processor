package httpapi

import (
	"net/http"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func buildHTTPServerBundleFromModules(port int, cfg *config.Config, modules []kernelmodule.Module) (*http.Server, []httproute.Descriptor, error) {
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, nil, err
	}
	server, routes := bundle.buildServerBundle(port)
	return server, routes, nil
}

func buildRegisteredRoutesForModules(cfg *config.Config, modules []kernelmodule.Module) ([]httproute.Descriptor, error) {
	bundle, err := buildRuntimeBundleFromModules(cfg, modules)
	if err != nil {
		return nil, err
	}
	return bundle.routes, nil
}
