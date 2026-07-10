package sourcea1688

import (
	"task-processor/internal/core/config"
	"task-processor/internal/kernel/module"
)

type BuildResult struct {
	Handler *Handler
	Module  module.Module
}

type routeModule struct {
	handler *Handler
}

func BuildModule(service TaskCommandService) *BuildResult {
	handler := NewHandler(service)
	return &BuildResult{
		Handler: handler,
		Module:  NewHTTPModule(handler),
	}
}

func NewHTTPModule(handler *Handler) module.Module {
	return routeModule{handler: handler}
}

func (m routeModule) Name() string {
	return ModuleName
}

func (m routeModule) Enabled(*config.Config) bool {
	return true
}

func (m routeModule) Register(reg *module.Registry) error {
	reg.AddRoutes(AppendRouteDescriptors(nil, m.handler)...)
	return nil
}
