package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskTemporalSubmissionWiring struct {
	assembly                         taskSubmissionAssembly
	orchestrator                     taskSubmissionOrchestratorWiring
	startSheinPublishWorkflow        func(context.Context, SheinPublishWorkflowStartInput) error
	loadSheinPublishTask             func(context.Context, string) (*Task, *SheinPackage, error)
	validateSheinPublishFreshness    func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	retrySheinSensitiveWordSubmit    func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	rememberSheinSubmittedResolution func(*Task, string)
}

type taskTemporalSubmissionConfigWiring struct {
	temporal    taskTemporalSubmissionWiring
	persistence *taskTemporalSubmissionPersistenceService
}

type taskTemporalSubmissionCollaborators struct {
	lifecycle   *taskTemporalSubmissionLifecycleService
	flow        *taskTemporalSubmissionFlowService
	persistence *taskTemporalSubmissionPersistenceService
	refresh     *taskTemporalSubmissionRefreshService
}

type taskTemporalSubmissionCollaboratorWiring struct {
	service *service
	wiring  taskTemporalSubmissionWiring
}

func buildTaskTemporalSubmissionWiring(s *service) taskTemporalSubmissionWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return buildTaskTemporalSubmissionWiringWithAssembly(s, base.assembly)
}

func buildTaskTemporalSubmissionConfigWiringWithPersistence(
	s *service,
	persistence *taskTemporalSubmissionPersistenceService,
) taskTemporalSubmissionConfigWiring {
	config := buildTaskTemporalSubmissionConfigWiring(s)
	config.persistence = persistence
	return config
}

func buildTaskTemporalSubmissionConfigWiring(s *service) taskTemporalSubmissionConfigWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return taskTemporalSubmissionConfigWiring{
		temporal: buildTaskTemporalSubmissionWiringWithAssembly(s, base.assembly),
	}
}

func buildTaskTemporalSubmissionCollaboratorWiring(s *service) taskTemporalSubmissionCollaboratorWiring {
	return taskTemporalSubmissionCollaboratorWiring{
		service: s,
		wiring:  buildTaskTemporalSubmissionWiring(s),
	}
}

func (w taskTemporalSubmissionCollaboratorWiring) newLifecycle() *taskTemporalSubmissionLifecycleService {
	return newTaskTemporalSubmissionLifecycleService(buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(w.wiring))
}

func (w taskTemporalSubmissionCollaboratorWiring) newFlow(persistence *taskTemporalSubmissionPersistenceService) *taskTemporalSubmissionFlowService {
	return newTaskTemporalSubmissionFlowService(buildTaskTemporalSubmissionFlowServiceConfigWithWiring(w.wiring, persistence))
}

func (w taskTemporalSubmissionCollaboratorWiring) newPersistence() *taskTemporalSubmissionPersistenceService {
	return newTaskTemporalSubmissionPersistenceService(buildTaskTemporalSubmissionPersistenceServiceConfigWithWiring(w.wiring))
}

func (w taskTemporalSubmissionCollaboratorWiring) newRefresh(persistence *taskTemporalSubmissionPersistenceService) *taskTemporalSubmissionRefreshService {
	return newTaskTemporalSubmissionRefreshService(buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(w.wiring, persistence))
}

func (w taskTemporalSubmissionCollaboratorWiring) resolve(existing taskTemporalSubmissionCollaborators) taskTemporalSubmissionCollaborators {
	persistence := existing.persistence
	if persistence == nil {
		persistence = w.newPersistence()
	}
	lifecycle := existing.lifecycle
	if lifecycle == nil {
		lifecycle = w.newLifecycle()
	}
	flow := existing.flow
	if flow == nil {
		flow = w.newFlow(persistence)
	}
	refresh := existing.refresh
	if refresh == nil {
		refresh = w.newRefresh(persistence)
	}
	return taskTemporalSubmissionCollaborators{
		lifecycle:   lifecycle,
		flow:        flow,
		persistence: persistence,
		refresh:     refresh,
	}
}

func buildTaskTemporalSubmissionWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskTemporalSubmissionWiring {
	assembly = completeTaskSubmissionAssembly(s, assembly)
	return taskTemporalSubmissionWiring{
		assembly:     assembly,
		orchestrator: buildTaskSubmissionOrchestratorWiring(s, assembly.resolver),
		startSheinPublishWorkflow: func(ctx context.Context, in SheinPublishWorkflowStartInput) error {
			client, _ := resolveSubmissionWorkflowClient(s)
			return client.StartSheinPublish(ctx, in)
		},
		loadSheinPublishTask:             s.loadSheinPublishTaskForTemporal,
		validateSheinPublishFreshness:    s.validateSheinPublishFreshness,
		retrySheinSensitiveWordSubmit:    s.retrySheinSensitiveWordSubmit,
		rememberSheinSubmittedResolution: s.rememberSheinSubmittedResolution,
	}
}

func buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionLifecycleServiceConfig {
	return taskTemporalSubmissionLifecycleServiceConfig{
		startSheinPublishWorkflow:     wiring.startSheinPublishWorkflow,
		beginSheinSubmitLease:         wiring.orchestrator.recovery.beginSheinSubmitLease,
		loadSheinPublishTask:          wiring.loadSheinPublishTask,
		normalizeSheinSubmitPackage:   wiring.orchestrator.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness: wiring.validateSheinPublishFreshness,
		saveTaskResult:                wiring.assembly.repository.saveTaskResult,
		handleWorkflowStartFailure:    wiring.orchestrator.recovery.handleSheinWorkflowStartFailure,
		getTaskPreview:                wiring.assembly.preview.getTaskPreview,
	}
}

func buildTaskTemporalSubmissionFlowServiceConfigWithWiring(
	wiring taskTemporalSubmissionWiring,
	persistence *taskTemporalSubmissionPersistenceService,
) taskTemporalSubmissionFlowServiceConfig {
	return taskTemporalSubmissionFlowServiceConfig{
		loadSheinPublishTask:          wiring.loadSheinPublishTask,
		normalizeSheinSubmitPackage:   wiring.orchestrator.bindings.execution.normalizeSheinSubmitPackage,
		persistSheinSubmitPhase:       wiring.orchestrator.bindings.state.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:     wiring.orchestrator.bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:       wiring.orchestrator.bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:         wiring.orchestrator.bindings.resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:    wiring.orchestrator.bindings.execution.buildSheinSubmitProductAPI,
		executeSheinSubmitRemote:      wiring.orchestrator.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit: wiring.retrySheinSensitiveWordSubmit,
		persistence:                   persistence,
	}
}

func buildTaskTemporalSubmissionPersistenceServiceConfigWithWiring(wiring taskTemporalSubmissionWiring) taskTemporalSubmissionPersistenceServiceConfig {
	return taskTemporalSubmissionPersistenceServiceConfig{
		loadSheinPublishTask:                 wiring.loadSheinPublishTask,
		saveTaskResult:                       wiring.assembly.repository.saveTaskResult,
		persistSheinSubmitPhase:              wiring.orchestrator.bindings.state.persistSheinSubmitPhase,
		persistSuccessfulSheinSubmission:     wiring.orchestrator.bindings.state.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: wiring.orchestrator.bindings.state.recordSheinSubmissionFailureForState,
		rememberSheinSubmitted:               wiring.rememberSheinSubmittedResolution,
	}
}

func buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(
	wiring taskTemporalSubmissionWiring,
	persistence *taskTemporalSubmissionPersistenceService,
) taskTemporalSubmissionRefreshServiceConfig {
	return taskTemporalSubmissionRefreshServiceConfig{
		loadSheinPublishTask:           wiring.loadSheinPublishTask,
		buildSheinSubmitProductAPI:     wiring.orchestrator.bindings.execution.buildSheinSubmitProductAPI,
		persistSheinSubmitPhase:        wiring.orchestrator.bindings.state.persistSheinSubmitPhase,
		refreshSheinSubmitRemoteStatus: wiring.orchestrator.recovery.refreshSheinSubmitRemoteStatus,
		persistence:                    persistence,
	}
}
