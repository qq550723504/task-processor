package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"
	appruntime "task-processor/internal/app/runtime"
	kernelmodule "task-processor/internal/kernel/module"
)

type TemporalRuntimeBuildInput struct {
	Logger       *logrus.Logger
	Runtime      RuntimeDependencies
	ServiceInput BuildServiceInput
}

type TemporalRuntimeResult struct {
	Module        kernelmodule.Module
	WorkerService TemporalWorkerService
	Closers       []func() error
}

func BuildTemporalRuntime(input TemporalRuntimeBuildInput) (*TemporalRuntimeResult, error) {
	serviceInput := input.ServiceInput
	if serviceInput.Config == nil {
		serviceInput = buildRuntimeServiceInput(input.Logger, input.Runtime)
	}

	bundle, err := BuildService(serviceInput)
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
				return appruntime.StartListingKitSheinPublishTemporalWorker(bundle.TemporalWorkerService, serviceInput.Logger)
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
