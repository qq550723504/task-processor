package httpapi

import (
	"fmt"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

type httpModuleHandlers struct {
	product          productRouteHandler
	image            imageRouteHandler
	amazonListing    amazonListingRouteHandler
	listingKit       listingKitRouteHandler
	promptTemplate   promptTemplateRouteHandler
	promptModule     kernelmodule.Module
	studioSession    studioSessionRouteHandler
	sheinLoginModule kernelmodule.Module
	sheinLogin       sheinLoginRouteHandler
	sdsLoginModule   kernelmodule.Module
	sdsLogin         sdsLoginRouteHandler
	taskRPCModule    kernelmodule.Module
	taskRPC          taskRPCRouteHandler
	sdsCatalog       sdsCatalogRouteHandler
	sdsModule        kernelmodule.Module
}

type httpModule struct {
	name     string
	enabled  func(cfg *config.Config) bool
	register func(reg *kernelmodule.Registry) error
}

func (m httpModule) Name() string {
	return m.name
}

func (m httpModule) Enabled(cfg *config.Config) bool {
	if m.enabled != nil {
		return m.enabled(cfg)
	}
	return true
}

func (m httpModule) Register(reg *kernelmodule.Registry) error {
	if m.register == nil {
		return fmt.Errorf("http module %s has no registrar", m.name)
	}
	return m.register(reg)
}
