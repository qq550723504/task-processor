package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskManagedSubmissionWiring struct {
	assembly                         taskSubmissionAssembly
	orchestrator                     taskSubmissionOrchestratorWiring
	buildSheinSubmitOtherAPI         func(context.Context, *Task) (sheinother.OtherAPI, error)
	validateSheinPublishFreshness    func(context.Context, *Task, *SheinPackage, string) (*SheinSubmitReadiness, error)
	retrySheinSensitiveWordSubmit    func(context.Context, string, *SheinPackage, string, string, sheinproduct.ProductAPI, *sheinproduct.Product, *sheinpub.SubmissionResponse, error) (*sheinpub.SubmissionResponse, error, bool)
	rememberSheinSubmittedResolution func(*Task, string)
}

type taskManagedSubmissionConfigWiring struct {
	support taskSubmissionSupportWiring
	managed taskManagedSubmissionWiring
}

type taskManagedSubmissionCollaboratorWiring struct {
	service  *service
	assembly taskSubmissionAssembly
	support  taskSubmissionSupportWiring
}

type taskManagedSubmissionCollaborators struct {
	recovery   *taskSubmissionRecoveryService
	direct     *taskDirectSubmissionService
	refresh    *taskSubmissionRefreshService
	submission *taskSubmissionService
}

func buildTaskManagedSubmissionWiring(s *service) taskManagedSubmissionWiring {
	assembly := buildTaskSubmissionAssembly(s)
	return buildTaskManagedSubmissionWiringWithAssembly(s, assembly)
}

func buildTaskManagedSubmissionWiringWithAssembly(s *service, assembly taskSubmissionAssembly) taskManagedSubmissionWiring {
	return buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s, assembly, s.taskSubmissionRecoveryOrDefault())
}

func buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s *service, assembly taskSubmissionAssembly, recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {
	assembly = completeTaskSubmissionAssembly(s, assembly)
	return taskManagedSubmissionWiring{
		assembly:                         assembly,
		orchestrator:                     buildTaskSubmissionOrchestratorWiringWithRecovery(s, assembly.resolver, recovery),
		buildSheinSubmitOtherAPI:         s.buildSheinSubmitOtherAPI,
		validateSheinPublishFreshness:    s.validateSheinPublishFreshness,
		retrySheinSensitiveWordSubmit:    s.retrySheinSensitiveWordSubmit,
		rememberSheinSubmittedResolution: s.rememberSheinSubmittedResolution,
	}
}

func buildTaskManagedSubmissionCollaboratorWiring(s *service) taskManagedSubmissionCollaboratorWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return taskManagedSubmissionCollaboratorWiring{
		service:  s,
		assembly: base.assembly,
		support:  base.support,
	}
}

func buildTaskManagedSubmissionConfigWiringWithRecovery(s *service, recovery *taskSubmissionRecoveryService) taskManagedSubmissionConfigWiring {
	base := buildTaskSubmissionBaseWiring(s)
	return taskManagedSubmissionConfigWiring{
		support: base.support,
		managed: buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(s, base.assembly, recovery),
	}
}

func (w taskManagedSubmissionCollaboratorWiring) newRecovery() *taskSubmissionRecoveryService {
	return newTaskSubmissionRecoveryService(buildTaskSubmissionRecoveryServiceConfigWithAssembly(w.service, w.assembly))
}

func (w taskManagedSubmissionCollaboratorWiring) buildManaged(recovery *taskSubmissionRecoveryService) taskManagedSubmissionWiring {
	return buildTaskManagedSubmissionWiringWithAssemblyAndRecovery(w.service, w.assembly, recovery)
}

func (w taskManagedSubmissionCollaboratorWiring) newDirect(managed taskManagedSubmissionWiring) *taskDirectSubmissionService {
	return newTaskDirectSubmissionService(buildTaskDirectSubmissionServiceConfigWithWiring(
		managed,
	))
}

func (w taskManagedSubmissionCollaboratorWiring) newRefresh(managed taskManagedSubmissionWiring) *taskSubmissionRefreshService {
	return newTaskSubmissionRefreshService(buildTaskSubmissionRefreshServiceConfigWithWiring(
		managed,
	))
}

func (w taskManagedSubmissionCollaboratorWiring) newSubmission(recovery *taskSubmissionRecoveryService, direct *taskDirectSubmissionService) *taskSubmissionService {
	return newTaskSubmissionService(buildTaskSubmissionServiceConfigWithSupportAndCollaborators(
		w.support,
		w.service,
		recovery,
		direct,
	))
}

func (w taskManagedSubmissionCollaboratorWiring) resolve(existing taskManagedSubmissionCollaborators) taskManagedSubmissionCollaborators {
	recovery := existing.recovery
	if recovery == nil {
		recovery = w.newRecovery()
	}
	managed := w.buildManaged(recovery)
	direct := existing.direct
	if direct == nil {
		direct = w.newDirect(managed)
	}
	refresh := existing.refresh
	if refresh == nil {
		refresh = w.newRefresh(managed)
	}
	submission := existing.submission
	if submission == nil {
		submission = w.newSubmission(recovery, direct)
	}
	return taskManagedSubmissionCollaborators{
		recovery:   recovery,
		direct:     direct,
		refresh:    refresh,
		submission: submission,
	}
}

func buildTaskSubmissionRecoveryServiceConfigWithAssembly(s *service, assembly taskSubmissionAssembly) taskSubmissionRecoveryServiceConfig {
	return taskSubmissionRecoveryServiceConfig{
		repo:                        assembly.repository.repo,
		buildTaskPreview:            assembly.preview.buildTaskPreview,
		buildSheinSubmitProductAPI:  assembly.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: assembly.bindings.state.persistSuccessfulSheinSubmission,
		recordSubmissionFailure:     assembly.bindings.state.recordSheinSubmissionFailureForState,
		resolveRemoteStatusCallback: resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfigWithSupportAndCollaborators(
	support taskSubmissionSupportWiring,
	s *service,
	recovery *taskSubmissionRecoveryService,
	direct *taskDirectSubmissionService,
) taskSubmissionServiceConfig {
	return taskSubmissionServiceConfig{
		repo:                            support.repo,
		lockSubmit:                      buildTaskSubmissionLockSubmit(s),
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		recovery:                        recovery,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           direct.submitSheinTaskDirect,
	}
}

func buildTaskSubmissionRefreshServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskSubmissionRefreshServiceConfig {
	return taskSubmissionRefreshServiceConfig{
		repo:                       wiring.assembly.repository.repo,
		lockSubmit:                 wiring.orchestrator.lockSubmit,
		buildTaskPreview:           wiring.assembly.preview.buildTaskPreview,
		buildSheinSubmitProductAPI: wiring.orchestrator.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   wiring.buildSheinSubmitOtherAPI,
		recovery:                   wiring.orchestrator.recovery,
		resolveRemoteStatus:        resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfigWithWiring(wiring taskManagedSubmissionWiring) taskDirectSubmissionServiceConfig {
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     wiring.assembly.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   wiring.validateSheinPublishFreshness,
		failSheinDirectSubmit:           wiring.assembly.bindings.state.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      wiring.assembly.bindings.execution.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   wiring.assembly.bindings.state.persistSheinDirectSubmitPhase,
		prepareSheinSubmitProduct:       wiring.assembly.bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:         wiring.assembly.bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:           wiring.assembly.bindings.resolver.resolveSubmitSettings,
		preValidateSheinSubmitProduct:   wiring.assembly.bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:        wiring.assembly.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:   wiring.retrySheinSensitiveWordSubmit,
		persistSuccessfulDirectResponse: wiring.assembly.bindings.state.persistSuccessfulSheinDirectResponse,
		finishSheinDirectSubmitAttempt:  wiring.assembly.bindings.state.finishSheinDirectSubmitAttempt,
		buildTaskPreview:                wiring.assembly.preview.buildTaskPreview,
	}
}
