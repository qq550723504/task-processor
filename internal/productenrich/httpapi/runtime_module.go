package httpapi

import (
	"task-processor/internal/core/config"
	module "task-processor/internal/kernel/module"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

const (
	productWorkerPoolName = "product_enrich"
	imageWorkerPoolName   = "product_image"
)

type runtimeModule struct {
	productModule *Module
	imageModule   *productimagehttpapi.Module
}

func NewRuntimeModule(productModule *Module, imageModule *productimagehttpapi.Module) module.Module {
	return runtimeModule{
		productModule: productModule,
		imageModule:   imageModule,
	}
}

func (m runtimeModule) Name() string {
	return httpModuleName
}

func (runtimeModule) Enabled(*config.Config) bool {
	return true
}

func (m runtimeModule) Register(reg *module.Registry) error {
	if m.productModule == nil {
		return nil
	}

	var imageHandler productimagehttpapi.RouteHandler
	if m.imageModule != nil {
		imageHandler = m.imageModule.Handler
	}
	reg.AddRoutes(AppendProductRouteDescriptors(nil, m.productModule.Handler, imageHandler)...)
	if m.productModule.Pool != nil {
		if err := reg.AddWorkerPool(productWorkerPoolName, m.productModule.Pool); err != nil {
			return err
		}
	}
	if m.imageModule != nil && m.imageModule.Pool != nil {
		if err := reg.AddWorkerPool(imageWorkerPoolName, m.imageModule.Pool); err != nil {
			return err
		}
	}
	return nil
}
