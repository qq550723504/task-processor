package httpapi

import (
	"task-processor/internal/core/config"
	module "task-processor/internal/kernel/module"
)

const workerPoolName = "amazon_listing"

type runtimeModule struct {
	built *Module
}

func NewRuntimeModule(built *Module) module.Module {
	return runtimeModule{built: built}
}

func (runtimeModule) Name() string {
	return httpModuleName
}

func (runtimeModule) Enabled(*config.Config) bool {
	return true
}

func (m runtimeModule) Register(reg *module.Registry) error {
	if m.built == nil {
		return nil
	}
	reg.AddRoutes(AppendRouteDescriptors(nil, m.built.Handler)...)
	if m.built.Pool != nil {
		if err := reg.AddWorkerPool(workerPoolName, m.built.Pool); err != nil {
			return err
		}
	}
	return nil
}
