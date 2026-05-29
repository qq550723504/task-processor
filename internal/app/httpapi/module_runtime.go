package httpapi

import (
	"fmt"

	kernelmodule "task-processor/internal/kernel/module"
)

func buildRegisteredRoutes(handlers httpModuleHandlers) ([]routeDescriptor, error) {
	reg := kernelmodule.NewRegistry()
	modules := []kernelmodule.Module{
		newCoreHTTPModule(),
		newProductHTTPModule(handlers),
		newAmazonListingHTTPModule(handlers),
		newListingKitHTTPModule(handlers),
		newOpsHTTPModule(handlers),
	}

	for _, module := range modules {
		if err := module.Register(reg); err != nil {
			return nil, fmt.Errorf("register module %s: %w", module.Name(), err)
		}
	}

	return reg.Routes(), nil
}
