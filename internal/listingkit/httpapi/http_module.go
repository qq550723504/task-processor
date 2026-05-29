package httpapi

import (
	"task-processor/internal/core/config"
	"task-processor/internal/kernel/module"
	"task-processor/internal/listingkit"
)

const httpModuleName = "listing-kit"

type httpModule struct {
	register func(reg *module.Registry) error
}

func NewHTTPModule(handler RouteHandler, promptTemplateHandler PromptTemplateRouteHandler, studioSessionHandler listingkit.StudioSessionHandler) module.Module {
	return httpModule{
		register: func(reg *module.Registry) error {
			routes := AppendRouteDescriptors(nil, handler)
			routes = AppendPromptTemplateRouteDescriptors(routes, promptTemplateHandler)
			routes = AppendStudioSessionRouteDescriptors(routes, studioSessionHandler)
			reg.AddRoutes(routes...)
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
