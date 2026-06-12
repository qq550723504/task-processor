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
	repository := buildTaskSubmissionRepositoryWiring(s)
	bindings := buildTaskSubmissionBindings(s, nil)
	return taskSubmissionRecoveryServiceConfig{
		repo:                        repository.repo,
		buildTaskPreview:            s.buildTaskPreview,
		buildSheinSubmitProductAPI:  bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: bindings.state.persistSuccessfulSheinSubmission,
		recordSubmissionFailure:     bindings.state.recordSheinSubmissionFailureForState,
		resolveRemoteStatusCallback: s.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {
	repository := buildTaskSubmissionRepositoryWiring(s)
	direct := s.taskDirectSubmissionOrDefault()
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskSubmissionServiceConfig{
		repo:                            repository.repo,
		lockSubmit:                      buildTaskSubmissionLockSubmit(s),
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          recovery.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           direct.submitSheinTaskDirect,
	}
}

func buildTaskSubmissionRefreshServiceConfig(s *service) taskSubmissionRefreshServiceConfig {
	repository := buildTaskSubmissionRepositoryWiring(s)
	wiring := buildTaskSubmissionOrchestratorWiring(s, nil)
	return taskSubmissionRefreshServiceConfig{
		repo:                       repository.repo,
		lockSubmit:                 wiring.lockSubmit,
		buildTaskPreview:           s.buildTaskPreview,
		buildSheinSubmitProductAPI: wiring.bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   s.buildSheinSubmitOtherAPI,
		mutateTaskResult:           wiring.recovery.mutateTaskResult,
		resolveRemoteStatus:        wiring.recovery.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfig(s *service) taskDirectSubmissionServiceConfig {
	bindings := buildTaskSubmissionBindings(s, buildSubmitRuntimeContextResolver(s))
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   s.validateSheinPublishFreshness,
		failSheinDirectSubmit:           bindings.state.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      bindings.execution.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   bindings.state.persistSheinDirectSubmitPhase,
		prepareSheinSubmitProduct:       bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:         bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:           bindings.resolver.resolveSubmitSettings,
		preValidateSheinSubmitProduct:   bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:        bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:   s.retrySheinSensitiveWordSubmit,
		persistSuccessfulDirectResponse: bindings.state.persistSuccessfulSheinDirectResponse,
		finishSheinDirectSubmitAttempt:  bindings.state.finishSheinDirectSubmitAttempt,
		buildTaskPreview:                s.buildTaskPreview,
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
	repository := buildTaskSubmissionRepositoryWiring(s)
	resolver := buildSubmitRuntimeContextResolver(s)
	wiring := buildTaskSubmissionOrchestratorWiring(s, resolver)
	return taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(ctx context.Context, in SheinPublishWorkflowStartInput) error {
			client, _ := resolveSubmissionWorkflowClient(s)
			return client.StartSheinPublish(ctx, in)
		},
		beginSheinSubmitLease:                wiring.recovery.beginSheinSubmitLease,
		loadSheinPublishTask:                 s.loadSheinPublishTaskForTemporal,
		normalizeSheinSubmitPackage:          wiring.bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       repository.saveTaskResult,
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
		getTaskPreview:                       s.GetTaskPreview,
	}
}
