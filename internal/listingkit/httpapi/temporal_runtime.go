package httpapi

import (
	"fmt"

	appruntime "task-processor/internal/app/runtime"
	kernelmodule "task-processor/internal/kernel/module"
)

type TemporalRuntimeBuildInput struct {
	ServiceInput BuildServiceInput
}

type TemporalRuntimeResult struct {
	Module        kernelmodule.Module
	WorkerService TemporalWorkerService
	Closers       []func() error
}

func BuildTemporalRuntime(input TemporalRuntimeBuildInput) (*TemporalRuntimeResult, error) {
	bundle, err := BuildService(input.ServiceInput)
	if err != nil {
		return nil, err
	}
	if bundle == nil || bundle.TemporalWorkerService == nil {
		return nil, nil
	}

	return &TemporalRuntimeResult{
		Module: buildTemporalModule(temporalModuleInput{
			Service: bundle.runtime.service,
			Starter: func() (func() error, error) {
				return appruntime.StartListingKitSheinPublishTemporalWorker(bundle.TemporalWorkerService, input.ServiceInput.Logger)
			},
		}),
		WorkerService: bundle.TemporalWorkerService,
		Closers:       bundle.Closers,
	}, nil
}

func (r *TemporalRuntimeResult) Validate() error {
	if r == nil {
		return fmt.Errorf("temporal runtime result is nil")
	}
	if r.Module == nil {
		return fmt.Errorf("temporal runtime module is nil")
	}
	if r.WorkerService == nil {
		return fmt.Errorf("temporal worker service is nil")
	}
	return nil
}
