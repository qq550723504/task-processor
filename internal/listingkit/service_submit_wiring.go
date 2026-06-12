package listingkit

import "context"

func buildTaskRequeueServiceConfig(s *service) taskRequeueServiceConfig {
	return taskRequeueServiceConfig{
		repo: s.repo,
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
	}
}

func buildTaskRecoveryServiceConfig(s *service) taskRecoveryServiceConfig {
	return taskRecoveryServiceConfig{
		repo: s.repo,
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
	}
}

func buildTaskSubmissionRecoveryServiceConfig(s *service) taskSubmissionRecoveryServiceConfig {
	bindings := buildTaskSubmissionBindings(s, nil)
	return taskSubmissionRecoveryServiceConfig{
		repo:                        s.repo,
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
	direct := s.taskDirectSubmissionOrDefault()
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskSubmissionServiceConfig{
		repo: s.repo,
		lockSubmit: func(key string) func() {
			return s.submission.sheinSubmitLocks.Lock(key)
		},
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          recovery.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           direct.submitSheinTaskDirect,
	}
}

func buildTaskSubmissionRefreshServiceConfig(s *service) taskSubmissionRefreshServiceConfig {
	bindings := buildTaskSubmissionBindings(s, nil)
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskSubmissionRefreshServiceConfig{
		repo: s.repo,
		lockSubmit: func(key string) func() {
			return s.submission.sheinSubmitLocks.Lock(key)
		},
		buildTaskPreview:           s.buildTaskPreview,
		buildSheinSubmitProductAPI: bindings.execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   s.buildSheinSubmitOtherAPI,
		mutateTaskResult:           recovery.mutateTaskResult,
		resolveRemoteStatus:        recovery.resolveSheinSubmitRemoteStatus,
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
		sheinProductAPIBuilder:   s.sheinProductAPIBuilder,
		sheinImageAPIBuilder:     s.sheinImageAPIBuilder,
		sheinTranslateAPIBuilder: s.sheinTranslateAPIBuilder,
		sheinContentOptimizer:    s.sheinContentOptimizer,
		currentSheinPricingRule:  s.currentSheinPricingRule,
		resolveSheinStoreID:      resolver.resolveStoreID,
		resolveSubmitSettings:    resolver.resolveSubmitSettings,
	}
}

func buildTaskSubmissionStateServiceConfig(s *service) taskSubmissionStateServiceConfig {
	return taskSubmissionStateServiceConfig{
		repo:                   s.repo,
		rememberSheinSubmitted: s.rememberSheinSubmittedResolution,
	}
}

func buildTaskTemporalSubmissionAdapterConfig(s *service) taskTemporalSubmissionAdapterConfig {
	resolver := buildSubmitRuntimeContextResolver(s)
	bindings := buildTaskSubmissionBindings(s, resolver)
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskTemporalSubmissionAdapterConfig{
		startSheinPublishWorkflow: func(ctx context.Context, in SheinPublishWorkflowStartInput) error {
			return s.sheinPublishWorkflowClient.StartSheinPublish(ctx, in)
		},
		beginSheinSubmitLease:                recovery.beginSheinSubmitLease,
		loadSheinPublishTask:                 s.loadSheinPublishTaskForTemporal,
		normalizeSheinSubmitPackage:          bindings.execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       s.repo.SaveTaskResult,
		persistSheinSubmitPhase:              bindings.state.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            bindings.execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              bindings.execution.uploadSheinSubmitImages,
		resolveSubmitSettings:                bindings.resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           bindings.execution.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct:        bindings.execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:             bindings.execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:        s.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     bindings.state.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: bindings.state.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       recovery.refreshSheinSubmitRemoteStatus,
		handleWorkflowStartFailure:           recovery.handleSheinWorkflowStartFailure,
		rememberSheinSubmitted:               s.rememberSheinSubmittedResolution,
		getTaskPreview:                       s.GetTaskPreview,
	}
}
