package httpapi

import (
	"net/http"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
)

func (c httpFeatureComposition) runtimeModules() []kernelmodule.Module {
	return []kernelmodule.Module{
		newCoreHTTPModule(),
		c.productHTTPModule(),
		c.amazonListingHTTPModule(),
		c.listingKitHTTPModule(),
		c.productSourcingHTTPModule(),
		c.promptHTTPModule(),
		c.listingKitStudioHTTPModule(),
		c.sdsHTTPModule(),
		c.sheinLoginHTTPModule(),
		c.sdsLoginHTTPModule(),
	}
}

func (c httpFeatureComposition) routeModules() []kernelmodule.Module {
	modules := c.runtimeModules()
	modules = append(modules, c.taskRPCHTTPModule())
	return modules
}

func (c httpFeatureComposition) buildRuntimeBundle(cfg *config.Config) (runtimeBundle, error) {
	return buildRuntimeBundleFromModules(cfg, c.routeModules())
}

func (c httpFeatureComposition) buildServerBundle(port int, cfg *config.Config) (*http.Server, []httproute.Descriptor, error) {
	bundle, err := c.buildRuntimeBundle(cfg)
	if err != nil {
		return nil, nil, err
	}
	server, routes := bundle.buildServerBundle(port)
	return server, routes, nil
}

func (c httpFeatureComposition) promptHTTPModule() kernelmodule.Module {
	if c.promptModule == nil {
		return nil
	}
	return c.promptModule.Module
}

func (c httpFeatureComposition) productSourcingHTTPModule() kernelmodule.Module {
	if c.productSourcingModule == nil {
		return nil
	}
	return c.productSourcingModule.Module
}

func (c httpFeatureComposition) sdsHTTPModule() kernelmodule.Module {
	if c.sdsModule == nil {
		return nil
	}
	return c.sdsModule.Module
}

func (c httpFeatureComposition) taskRPCHTTPModule() kernelmodule.Module {
	if c.taskRPCResult == nil {
		return nil
	}
	return c.taskRPCResult.Module
}

func (c httpFeatureComposition) sheinLoginHTTPModule() kernelmodule.Module {
	if c.sheinLoginResult == nil {
		return nil
	}
	return c.sheinLoginResult.Module
}

func (c httpFeatureComposition) sdsLoginHTTPModule() kernelmodule.Module {
	if c.sdsLoginResult == nil {
		return nil
	}
	return c.sdsLoginResult.Module
}
