package httpapi

import (
	"task-processor/internal/core/config"
	module "task-processor/internal/kernel/module"
)

const imageWorkerPoolName = "product_image"

type runtimeModule struct {
	built *Module
}

func NewRuntimeModule(built *Module) module.Module {
	return runtimeModule{built: built}
}

func (runtimeModule) Name() string {
	return "product-image"
}

func (runtimeModule) Enabled(*config.Config) bool {
	return true
}

func (m runtimeModule) Register(reg *module.Registry) error {
	if m.built == nil || m.built.Pool == nil {
		return nil
	}
	return reg.AddWorkerPool(imageWorkerPoolName, m.built.Pool)
}
