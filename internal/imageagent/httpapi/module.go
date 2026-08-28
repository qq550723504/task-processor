package httpapi

import (
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

type BuildResult struct {
	Handler *Handler
	Module  kernelmodule.Module
	Closers []func() error
}

func BuildModule(application Application, options ...HandlerOption) (*BuildResult, error) {
	handler, err := NewHandler(application, options...)
	if err != nil {
		return nil, err
	}
	return &BuildResult{Handler: handler, Module: NewHTTPModule(handler)}, nil
}

func NewHTTPModule(handler *Handler) kernelmodule.Module { return routeModule{handler: handler} }

type routeModule struct{ handler *Handler }

func (routeModule) Name() string { return ModuleName }

func (m routeModule) Enabled(*config.Config) bool {
	return m.handler != nil && m.handler.application != nil
}

func (m routeModule) Register(registry *kernelmodule.Registry) error {
	registry.AddRoutes(AppendRouteDescriptors(nil, m.handler)...)
	return nil
}
