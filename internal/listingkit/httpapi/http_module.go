package httpapi

import (
	"task-processor/internal/core/config"
	"task-processor/internal/kernel/module"
	"task-processor/internal/listingkit"
)

const httpModuleName = "listing-kit"
const studioHTTPModuleName = "listing-kit-studio"

type httpModule struct {
	name     string
	register func(reg *module.Registry) error
}

func NewHTTPModule(handler RouteHandler) module.Module {
	return httpModule{
		name: httpModuleName,
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendRouteDescriptors(nil, handler)...)
			return nil
		},
	}
}

func NewStudioHTTPModule(handler listingkit.StudioSessionHandler) module.Module {
	return httpModule{
		name: studioHTTPModuleName,
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendStudioSessionRouteDescriptors(nil, handler)...)
			return nil
		},
	}
}

func (m httpModule) Name() string {
	if m.name != "" {
		return m.name
	}
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
