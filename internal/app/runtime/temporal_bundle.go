package runtime

import (
	"fmt"

	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

type TemporalRuntimeBundle struct {
	Workers []kernelmodule.NamedTemporalWorker
}

func BuildTemporalRuntimeBundleFromModules(cfg *config.Config, modules []kernelmodule.Module) (TemporalRuntimeBundle, error) {
	reg := kernelmodule.NewRegistry()

	for _, mod := range modules {
		if mod == nil {
			continue
		}
		if !mod.Enabled(cfg) {
			continue
		}
		if err := mod.Register(reg); err != nil {
			return TemporalRuntimeBundle{}, fmt.Errorf("register module %s: %w", mod.Name(), err)
		}
	}

	return TemporalRuntimeBundle{
		Workers: reg.TemporalWorkers(),
	}, nil
}

func (b TemporalRuntimeBundle) Start() ([]func() error, error) {
	closers := make([]func() error, 0, len(b.Workers))
	for _, worker := range b.Workers {
		if worker.Start == nil {
			continue
		}
		closeFn, err := worker.Start()
		if err != nil {
			for i := len(closers) - 1; i >= 0; i-- {
				if closers[i] != nil {
					_ = closers[i]()
				}
			}
			return nil, fmt.Errorf("start temporal worker %s: %w", worker.Name, err)
		}
		if closeFn != nil {
			closers = append(closers, closeFn)
		}
	}
	return closers, nil
}
