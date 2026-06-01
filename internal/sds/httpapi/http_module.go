package httpapi

import (
	"net/http"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/kernel/module"
)

type BuildResult struct {
	Handler HTTPRouteHandler
	Module  module.Module
}

const httpModuleName = "sds"

type routeModule struct {
	register func(reg *module.Registry) error
}

func BuildModule(logger *logrus.Logger, cfg *config.Config) *BuildResult {
	handler := BuildCatalogHandler(logger, cfg)
	return &BuildResult{
		Handler: handler,
		Module:  NewHTTPModule(handler),
	}
}

func NewHTTPModule(handler HTTPRouteHandler) module.Module {
	return routeModule{
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendRouteDescriptors(nil, handler)...)
			return nil
		},
	}
}

func (m routeModule) Name() string {
	return httpModuleName
}

func (routeModule) Enabled(*config.Config) bool {
	return true
}

func (m routeModule) Register(reg *module.Registry) error {
	if m.register != nil {
		return m.register(reg)
	}
	return nil
}

func AppendRouteDescriptors(routes []httproute.Descriptor, handler HTTPRouteHandler) []httproute.Descriptor {
	if handler == nil {
		return routes
	}
	return append(routes,
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds/products", Module: "sds", Handler: handler.ListSDSProducts},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds/products/:product_id", Module: "sds", Handler: handler.GetSDSProduct},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds/categories", Module: "sds", Handler: handler.ListSDSCategories},
		httproute.Descriptor{Method: http.MethodGet, Path: "/api/v1/sds/shipment-areas", Module: "sds", Handler: handler.ListSDSShipmentAreas},
	)
}
