package httpapi

import (
	"task-processor/internal/core/config"
	"task-processor/internal/kernel/module"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

const httpModuleName = "product"

type httpModule struct {
	register func(reg *module.Registry) error
}

func NewHTTPModule(productHandler productenrich.ProductHandler, imageHandler productimage.Handler) module.Module {
	return httpModule{
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendProductRouteDescriptors(nil, productHandler, imageHandler)...)
			return nil
		},
	}
}

func (m httpModule) Name() string {
	return httpModuleName
}

func (httpModule) Enabled(*config.Config) bool {
	return true
}

func (m httpModule) Register(reg *module.Registry) error {
	if m.register != nil {
		return m.register(reg)
	}
	return nil
}
