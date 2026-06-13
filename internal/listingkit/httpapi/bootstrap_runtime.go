package httpapi

import (
	"fmt"
	"time"

	appruntime "task-processor/internal/app/runtime"
	"task-processor/internal/httpbootstrap"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
)

type serviceRuntimeModules struct {
	task     taskModule
	admin    adminModule
	submit   submitModule
	temporal temporalModule
}

type serviceRuntimeAssembly struct {
	service             moduleService
	modules             serviceRuntimeModules
	handlerDependencies listingkitapi.HandlerDependencies
}

type moduleRuntimeAssembly struct {
	processor *listingkit.Processor
	pool      worker.WorkerPool
}

func prepareModuleRuntimeClosers(input BuildModuleInput, bundle *ServiceBundle) (_ *closerStack, err error) {
	closers := &closerStack{}
	closers.Add(bundle.runtime.closers...)
	if input.ShouldStartTemporalWorkerInProcess {
		temporalWorkerCloser, startErr := appruntime.StartListingKitSheinPublishTemporalWorker(bundle.runtime.temporalWorkerService, input.ServiceInput.Logger)
		if startErr != nil {
			return nil, fmt.Errorf("start listing kit shein publish temporal worker: %w", startErr)
		}
		closers.Add(temporalWorkerCloser)
	}
	return closers, nil
}

func assembleModuleRuntime(input BuildModuleInput, bundle *ServiceBundle) (*moduleRuntimeAssembly, error) {
	processor, err := listingkit.NewProcessor(bundle.runtime.service, bundle.runtime.taskRepository, input.ServiceInput.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create listing kit processor: %w", err)
	}
	pool := httpbootstrap.NewWorkerPool(processor, input.ServiceInput.Config)
	submitter := &httpbootstrap.PoolSubmitter{Pool: pool}
	bundle.runtime.service.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)
	return &moduleRuntimeAssembly{
		processor: processor,
		pool:      pool,
	}, nil
}

func createModuleRuntime(input BuildModuleInput, bundle *ServiceBundle, closers *closerStack) (*Module, error) {
	assembly, err := assembleModuleRuntime(input, bundle)
	if err != nil {
		return nil, err
	}
	startTaskRecoverySweep(input, bundle, closers)
	handler, err := listingkitapi.NewHandler(nil, buildHandlerOptions(bundle.runtime)...)
	if err != nil {
		return nil, fmt.Errorf("create listing kit handler: %w", err)
	}

	studioSessionHandler, err := listingkitapi.NewStudioSessionHandler(bundle.runtime.service)
	if err != nil {
		return nil, fmt.Errorf("create listing kit studio session handler: %w", err)
	}

	return &Module{
		Handler:              handler,
		StudioSessionHandler: studioSessionHandler,
		Pool:                 assembly.pool,
		Closers:              closers.Snapshot(),
	}, nil
}

func buildModuleRuntime(input BuildModuleInput, bundle *ServiceBundle) (_ *Module, err error) {
	closers, err := prepareModuleRuntimeClosers(input, bundle)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		_ = closers.Close()
	}()
	return createModuleRuntime(input, bundle, closers)
}

func buildServiceRuntimeModules(input BuildServiceInput, repositories *builtRepositories) serviceRuntimeModules {
	task := buildTaskModule(newTaskModuleInput(input, repositories))
	admin := buildAdminModule(newAdminModuleInput(repositories))
	submit := buildSubmitModule(newSubmitModuleInput(input, repositories))
	return serviceRuntimeModules{
		task:   task,
		admin:  admin,
		submit: submit,
	}
}

func assembleServiceRuntime(input BuildServiceInput, repositories *builtRepositories, closers *closerStack) (serviceRuntimeAssembly, error) {
	modules := buildServiceRuntimeModules(input, repositories)
	moduleSvc, err := buildModuleService(input, repositories, modules.submit, closers)
	if err != nil {
		return serviceRuntimeAssembly{}, err
	}
	modules.temporal = buildTemporalModule(temporalModuleInput{
		Service: moduleSvc,
	})
	return serviceRuntimeAssembly{
		service:             moduleSvc,
		modules:             modules,
		handlerDependencies: modules.task.handlerDependenciesWithAdmin(modules.admin),
	}, nil
}

func buildServiceRuntime(input BuildServiceInput, repositories *builtRepositories, closers *closerStack) (*ServiceBundle, error) {
	if input.Logger != nil {
		input.Logger.WithField("component", "listingkit/httpapi").Info("listingkit service runtime begin")
	}
	configureSheinSubmitPrepRuntime(repositories)
	runtimeAssemblyStartedAt := time.Now()
	assembly, err := assembleServiceRuntime(input, repositories, closers)
	if err != nil {
		return nil, err
	}
	if input.Logger != nil {
		input.Logger.WithField("component", "listingkit/httpapi").WithField("elapsed", time.Since(runtimeAssemblyStartedAt)).Info("listingkit service runtime assembled")
	}
	sheinSyncRuntimeStartedAt := time.Now()
	runtimeServices, err := buildSheinSyncRuntimeServices(input, repositories, closers)
	if err != nil {
		return nil, err
	}
	if input.Logger != nil {
		input.Logger.WithField("component", "listingkit/httpapi").WithField("elapsed", time.Since(sheinSyncRuntimeStartedAt)).Info("listingkit shein sync runtime ready")
	}
	return assembleServiceBundle(repositories, assembly.service, runtimeServices, assembly.modules.temporal.workerService, assembly.handlerDependencies, closers.Snapshot()), nil
}
