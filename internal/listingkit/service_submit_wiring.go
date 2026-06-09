package listingkit

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
	return taskSubmissionRecoveryServiceConfig{
		repo:                       s.repo,
		buildTaskPreview:           s.buildTaskPreview,
		buildSheinSubmitProductAPI: s.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:   s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:     s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: s.persistSuccessfulSheinSubmission,
		resolveRemoteStatusCallback: s.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {
	return taskSubmissionServiceConfig{
		repo: s.repo,
		lockSubmit: func(key string) func() {
			return s.submission.sheinSubmitLocks.Lock(key)
		},
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          s.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           s.submitSheinTaskDirect,
		buildTaskPreview:                s.buildTaskPreview,
		buildSheinSubmitProductAPI:      s.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:        s.buildSheinSubmitOtherAPI,
		mutateTaskResult:                s.mutateTaskResult,
		resolveRemoteStatus:             s.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfig(s *service) taskDirectSubmissionServiceConfig {
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     s.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   s.validateSheinPublishFreshness,
		failSheinDirectSubmit:           s.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      s.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   s.persistSheinDirectSubmitPhase,
		prepareSheinDirectSubmitProduct: s.prepareSheinDirectSubmitProduct,
		completeSheinDirectRemoteSubmit: s.completeSheinDirectRemoteSubmit,
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

func buildTaskTemporalSubmissionAdapterConfig(s *service) taskTemporalSubmissionAdapterConfig {
	resolver := buildSubmitRuntimeContextResolver(s)
	return taskTemporalSubmissionAdapterConfig{
		beginSheinSubmitLease:                s.beginSheinSubmitLease,
		loadSheinPublishTask:                 s.loadSheinPublishTask,
		normalizeSheinSubmitPackage:          s.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       s.repo.SaveTaskResult,
		persistSheinSubmitPhase:              s.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            s.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              s.uploadSheinSubmitImages,
		resolveSubmitSettings:                resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           s.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct:        s.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:             s.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:        s.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     s.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: s.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       s.refreshSheinSubmitRemoteStatus,
		rememberSheinSubmitted:               s.rememberSheinSubmittedResolution,
		getTaskPreview:                       s.GetTaskPreview,
	}
}
