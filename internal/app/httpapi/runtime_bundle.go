package httpapi

import (
	"fmt"
	"net/http"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/taskrpcapi"
)

type runtimeBundle struct {
	routes      []httproute.Descriptor
	workerPools []kernelmodule.NamedWorkerPool
}

func buildRuntimeBundleFromModules(cfg *config.Config, modules []kernelmodule.Module) (runtimeBundle, error) {
	reg := kernelmodule.NewRegistry()
	filtered := make([]kernelmodule.Module, 0, len(modules))

	for _, mod := range modules {
		if mod == nil {
			continue
		}
		if !mod.Enabled(cfg) {
			continue
		}
		if err := mod.Register(reg); err != nil {
			return runtimeBundle{}, fmt.Errorf("register module %s: %w", mod.Name(), err)
		}
		filtered = append(filtered, mod)
	}

	return runtimeBundle{
		routes:      reg.Routes(),
		workerPools: reg.WorkerPools(),
	}, nil
}

func (b runtimeBundle) buildServerBundle(port int) (*http.Server, []httproute.Descriptor) {
	return buildHTTPServerFromRoutes(port, b.routes), b.routes
}

func (b runtimeBundle) pools() []worker.WorkerPool {
	pools := make([]worker.WorkerPool, 0, len(b.workerPools))
	for _, item := range b.workerPools {
		if item.Pool == nil {
			continue
		}
		pools = append(pools, item.Pool)
	}
	return pools
}

func (b runtimeBundle) localTaskHealthProvider() taskrpcapi.LocalStatusProvider {
	pools := make(map[string]worker.WorkerPool, len(b.workerPools))
	for _, item := range b.workerPools {
		if item.Pool == nil {
			continue
		}
		pools[item.Name] = item.Pool
	}
	return buildLocalTaskHealthProvider(pools)
}
