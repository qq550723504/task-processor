package httpapi

import (
	"task-processor/internal/amazonlisting"
	"task-processor/internal/core/config"
	"task-processor/internal/kernel/module"
)

const httpModuleName = "amazon-listing"

type httpModule struct {
	register func(reg *module.Registry) error
}

func NewHTTPModule(handler amazonlisting.Handler) module.Module {
	return httpModule{
		register: func(reg *module.Registry) error {
			reg.AddRoutes(AppendRouteDescriptors(nil, handler)...)
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
