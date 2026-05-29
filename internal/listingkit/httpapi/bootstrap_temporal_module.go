package httpapi

import (
	"task-processor/internal/core/config"
	kernelmodule "task-processor/internal/kernel/module"
)

const (
	temporalRuntimeModuleName = "listingkit-temporal"
	temporalWorkerName        = "listingkit_shein_publish"
)

type temporalModuleInput struct {
	Service moduleService
	Starter kernelmodule.TemporalWorkerStarter
}

type temporalModule struct {
	workerService TemporalWorkerService
	starter       kernelmodule.TemporalWorkerStarter
}

func buildTemporalModule(in temporalModuleInput) temporalModule {
	return temporalModule{
		workerService: in.Service,
		starter:       in.Starter,
	}
}

func (temporalModule) Name() string {
	return temporalRuntimeModuleName
}

func (temporalModule) Enabled(*config.Config) bool {
	return true
}

func (m temporalModule) Register(reg *kernelmodule.Registry) error {
	if reg == nil || m.workerService == nil || m.starter == nil {
		return nil
	}

	for _, name := range []string{"PublishWorkflow", "StandardProductWorkflow", "PlatformAdaptWorkflow"} {
		if err := reg.RegisterWorkflowHandler(staticWorkflowHandler{name: name}); err != nil {
			return err
		}
	}

	return reg.AddTemporalWorker(temporalWorkerName, m.starter)
}

type staticWorkflowHandler struct {
	name string
}

func (h staticWorkflowHandler) WorkflowName() string {
	return h.name
}

func (h staticWorkflowHandler) RegisterWorkflow(reg *kernelmodule.WorkflowRegistry) error {
	if reg == nil {
		return nil
	}
	return reg.Register(h.name)
}
