package listingkit

func buildTaskRequeueServiceConfig(s *service) taskRequeueServiceConfig {
	return buildTaskRequeueServiceConfigWithWiring(buildTaskSubmitterWiring(s))
}

func buildTaskRequeueServiceConfigWithWiring(wiring taskSubmitterWiring) taskRequeueServiceConfig {
	return taskRequeueServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
}

func buildTaskRecoveryServiceConfig(s *service) taskRecoveryServiceConfig {
	return buildTaskRecoveryServiceConfigWithWiring(buildTaskSubmitterWiring(s))
}

func buildTaskRecoveryServiceConfigWithWiring(wiring taskSubmitterWiring) taskRecoveryServiceConfig {
	return taskRecoveryServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
}

func buildTaskSubmissionRecoveryServiceConfig(s *service) taskSubmissionRecoveryServiceConfig {
	base := buildTaskSubmissionBaseWiring(s)
	return buildTaskSubmissionRecoveryServiceConfigWithAssembly(s, base.assembly)
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

func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {
	return buildTaskSubmissionServiceConfigWithCollaborators(s, s.taskSubmissionRecoveryOrDefault(), s.taskDirectSubmissionOrDefault())
}

func buildTaskSubmissionServiceConfigWithCollaborators(
	s *service,
	recovery *taskSubmissionRecoveryService,
	direct *taskDirectSubmissionService,
) taskSubmissionServiceConfig {
	config := buildTaskManagedSubmissionConfigWiringWithRecovery(s, recovery)
	return buildTaskSubmissionServiceConfigWithSupportAndCollaborators(config.support, s, recovery, direct)
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

func buildTaskSubmissionRefreshServiceConfig(s *service) taskSubmissionRefreshServiceConfig {
	return buildTaskSubmissionRefreshServiceConfigWithRecovery(s, s.taskSubmissionRecoveryOrDefault())
}

func buildTaskSubmissionRefreshServiceConfigWithRecovery(s *service, recovery *taskSubmissionRecoveryService) taskSubmissionRefreshServiceConfig {
	config := buildTaskManagedSubmissionConfigWiringWithRecovery(s, recovery)
	return buildTaskSubmissionRefreshServiceConfigWithWiring(config.managed)
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

func buildTaskDirectSubmissionServiceConfig(s *service) taskDirectSubmissionServiceConfig {
	return buildTaskDirectSubmissionServiceConfigWithRecovery(s, s.taskSubmissionRecoveryOrDefault())
}

func buildTaskDirectSubmissionServiceConfigWithRecovery(s *service, recovery *taskSubmissionRecoveryService) taskDirectSubmissionServiceConfig {
	config := buildTaskManagedSubmissionConfigWiringWithRecovery(s, recovery)
	return buildTaskDirectSubmissionServiceConfigWithWiring(config.managed)
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

func buildTaskSubmissionExecutionServiceConfig(s *service) taskSubmissionExecutionServiceConfig {
	return buildTaskSubmissionExecutionServiceConfigWithSupport(buildTaskSubmissionSupportWiring(s))
}

func buildTaskSubmissionExecutionServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionExecutionServiceConfig {
	return taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder:   wiring.sheinProductAPIBuilder,
		sheinImageAPIBuilder:     wiring.sheinImageAPIBuilder,
		sheinTranslateAPIBuilder: wiring.sheinTranslateAPIBuilder,
		sheinContentOptimizer:    wiring.sheinContentOptimizer,
		currentSheinPricingRule:  wiring.currentSheinPricingRule,
		resolveSheinStoreID:      wiring.resolveSheinStoreID,
		resolveSubmitSettings:    wiring.resolveSubmitSettings,
	}
}

func buildTaskSubmissionStateServiceConfig(s *service) taskSubmissionStateServiceConfig {
	return buildTaskSubmissionStateServiceConfigWithSupport(buildTaskSubmissionSupportWiring(s))
}

func buildTaskSubmissionStateServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionStateServiceConfig {
	return taskSubmissionStateServiceConfig{
		repo:                   wiring.repo,
		rememberSheinSubmitted: wiring.rememberSheinSubmitted,
	}
}

func buildTaskTemporalSubmissionLifecycleServiceConfig(s *service) taskTemporalSubmissionLifecycleServiceConfig {
	config := buildTaskTemporalSubmissionConfigWiring(s)
	return buildTaskTemporalSubmissionLifecycleServiceConfigWithWiring(config.temporal)
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

func buildTaskTemporalSubmissionFlowServiceConfig(s *service) taskTemporalSubmissionFlowServiceConfig {
	return buildTaskTemporalSubmissionFlowServiceConfigWithPersistence(s, s.taskTemporalSubmissionPersistenceOrDefault())
}

func buildTaskTemporalSubmissionFlowServiceConfigWithPersistence(s *service, persistence *taskTemporalSubmissionPersistenceService) taskTemporalSubmissionFlowServiceConfig {
	config := buildTaskTemporalSubmissionConfigWiringWithPersistence(s, persistence)
	return buildTaskTemporalSubmissionFlowServiceConfigWithWiring(config.temporal, config.persistence)
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
		preValidateSheinSubmitProduct: wiring.orchestrator.bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:      wiring.orchestrator.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit: wiring.retrySheinSensitiveWordSubmit,
		persistence:                   persistence,
	}
}

func buildTaskTemporalSubmissionPersistenceServiceConfig(s *service) taskTemporalSubmissionPersistenceServiceConfig {
	config := buildTaskTemporalSubmissionConfigWiring(s)
	return buildTaskTemporalSubmissionPersistenceServiceConfigWithWiring(config.temporal)
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

func buildTaskTemporalSubmissionServiceConfig(s *service) taskTemporalSubmissionServiceConfig {
	wiring := buildTaskTemporalSubmissionFacadeWiring(s)
	return buildTaskTemporalSubmissionServiceConfigWithCollaborators(
		wiring.lifecycle,
		wiring.flow,
		wiring.persistence,
		wiring.refresh,
	)
}

func buildTaskTemporalSubmissionServiceConfigWithCollaborators(
	lifecycle *taskTemporalSubmissionLifecycleService,
	flow *taskTemporalSubmissionFlowService,
	persistence *taskTemporalSubmissionPersistenceService,
	refresh *taskTemporalSubmissionRefreshService,
) taskTemporalSubmissionServiceConfig {
	return taskTemporalSubmissionServiceConfig{
		lifecycle:   lifecycle,
		flow:        flow,
		persistence: persistence,
		refresh:     refresh,
	}
}

func buildTaskTemporalSubmissionRefreshServiceConfig(s *service) taskTemporalSubmissionRefreshServiceConfig {
	return buildTaskTemporalSubmissionRefreshServiceConfigWithPersistence(s, s.taskTemporalSubmissionPersistenceOrDefault())
}

func buildTaskTemporalSubmissionRefreshServiceConfigWithPersistence(s *service, persistence *taskTemporalSubmissionPersistenceService) taskTemporalSubmissionRefreshServiceConfig {
	config := buildTaskTemporalSubmissionConfigWiringWithPersistence(s, persistence)
	return buildTaskTemporalSubmissionRefreshServiceConfigWithWiring(config.temporal, config.persistence)
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
