package listingkit

import "context"

func buildTaskRequeueServiceConfig(s *service) taskRequeueServiceConfig {
	wiring := buildTaskSubmitterWiring(s)
	return taskRequeueServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
}

func buildTaskRecoveryServiceConfig(s *service) taskRecoveryServiceConfig {
	wiring := buildTaskSubmitterWiring(s)
	return taskRecoveryServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
}

func buildTaskSubmissionRecoveryServiceConfig(s *service) taskSubmissionRecoveryServiceConfig {
	assembly := buildTaskSubmissionAssembly(s)
	return taskSubmissionRecoveryServiceConfig{
		repo:                        assembly.repository.repo,
		buildTaskPreview:            assembly.preview.buildTaskPreview,
		buildSheinSubmitProductAPI:  assembly.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: assembly.bindings.state.persistSuccessfulSheinSubmission,
		recordSubmissionFailure:     assembly.bindings.state.recordSheinSubmissionFailureForState,
		resolveRemoteStatusCallback: s.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {
	repository := buildTaskSubmissionRepositoryWiring(s)
	wiring := buildTaskSubmissionServiceWiring(s)
	return taskSubmissionServiceConfig{
		repo:                            repository.repo,
		lockSubmit:                      wiring.lockSubmit,
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          wiring.recovery.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           wiring.direct.submitSheinTaskDirect,
	}
}

func buildTaskSubmissionRefreshServiceConfig(s *service) taskSubmissionRefreshServiceConfig {
	assembly := buildTaskSubmissionAssembly(s)
	wiring := buildTaskSubmissionOrchestratorWiring(s, assembly.resolver)
	return taskSubmissionRefreshServiceConfig{
		repo:                       assembly.repository.repo,
		lockSubmit:                 wiring.lockSubmit,
		buildTaskPreview:           assembly.preview.buildTaskPreview,
		buildSheinSubmitProductAPI: wiring.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   s.buildSheinSubmitOtherAPI,
		mutateTaskResult:           wiring.recovery.mutateTaskResult,
		resolveRemoteStatus:        wiring.recovery.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfig(s *service) taskDirectSubmissionServiceConfig {
	assembly := buildTaskSubmissionAssembly(s)
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     assembly.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   s.validateSheinPublishFreshness,
		failSheinDirectSubmit:           assembly.bindings.state.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      assembly.bindings.execution.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   assembly.bindings.state.persistSheinDirectSubmitPhase,
		prepareSheinSubmitProduct:       assembly.bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:         assembly.bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:           assembly.bindings.resolver.resolveSubmitSettings,
		preValidateSheinSubmitProduct:   assembly.bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:        assembly.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:   s.retrySheinSensitiveWordSubmit,
		persistSuccessfulDirectResponse: assembly.bindings.state.persistSuccessfulSheinDirectResponse,
		finishSheinDirectSubmitAttempt:  assembly.bindings.state.finishSheinDirectSubmitAttempt,
		buildTaskPreview:                assembly.preview.buildTaskPreview,
	}
}

func buildTaskSubmissionExecutionServiceConfig(s *service) taskSubmissionExecutionServiceConfig {
	resolver := buildSubmitRuntimeContextResolver(s)
	return taskSubmissionExecutionServiceConfig{
		sheinProductAPIBuilder:   resolveSubmissionProductAPIBuilder(s),
		sheinImageAPIBuilder:     resolveSubmissionImageAPIBuilder(s),
		sheinTranslateAPIBuilder: resolveSubmissionTranslateAPIBuilder(s),
		sheinContentOptimizer:    resolveSubmissionContentOptimizer(s),
		currentSheinPricingRule:  s.currentSheinPricingRule,
		resolveSheinStoreID:      resolver.resolveStoreID,
		resolveSubmitSettings:    resolver.resolveSubmitSettings,
	}
}

func buildTaskSubmissionStateServiceConfig(s *service) taskSubmissionStateServiceConfig {
	repository := buildTaskSubmissionRepositoryWiring(s)
	return taskSubmissionStateServiceConfig{
		repo:                   repository.repo,
		rememberSheinSubmitted: s.rememberSheinSubmittedResolution,
	}
}

func buildTaskTemporalSubmissionAdapterConfig(s *service) taskTemporalSubmissionAdapterConfig {
	assembly := buildTaskSubmissionAssembly(s)
	wiring := buildTaskSubmissionOrchestratorWiring(s, assembly.resolver)
	return taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(ctx context.Context, in SheinPublishWorkflowStartInput) error {
			client, _ := resolveSubmissionWorkflowClient(s)
			return client.StartSheinPublish(ctx, in)
		},
		beginSheinSubmitLease:                wiring.recovery.beginSheinSubmitLease,
		loadSheinPublishTask:                 s.loadSheinPublishTaskForTemporal,
		normalizeSheinSubmitPackage:          wiring.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       assembly.repository.saveTaskResult,
		persistSheinSubmitPhase:              wiring.bindings.state.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            wiring.bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              wiring.bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:                wiring.bindings.resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           wiring.bindings.execution.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct:        wiring.bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:             wiring.bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:        s.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     wiring.bindings.state.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: wiring.bindings.state.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       wiring.recovery.refreshSheinSubmitRemoteStatus,
		handleWorkflowStartFailure:           wiring.recovery.handleSheinWorkflowStartFailure,
		rememberSheinSubmitted:               s.rememberSheinSubmittedResolution,
		getTaskPreview:                       assembly.preview.getTaskPreview,
	}
}
