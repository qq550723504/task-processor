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
	state := s.taskSubmissionStateOrDefault()
	execution := s.taskSubmissionExecutionOrDefault()
	return taskSubmissionRecoveryServiceConfig{
		repo:                        s.repo,
		buildTaskPreview:            s.buildTaskPreview,
		buildSheinSubmitProductAPI:  execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    s.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      s.rememberSheinSubmittedResolution,
		persistSuccessfulSubmission: state.persistSuccessfulSheinSubmission,
		resolveRemoteStatusCallback: s.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskSubmissionServiceConfig(s *service) taskSubmissionServiceConfig {
	execution := s.taskSubmissionExecutionOrDefault()
	direct := s.taskDirectSubmissionOrDefault()
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskSubmissionServiceConfig{
		repo: s.repo,
		lockSubmit: func(key string) func() {
			return s.submission.sheinSubmitLocks.Lock(key)
		},
		resolveDefaultSheinSubmitAction: s.resolveDefaultSheinSubmitAction,
		acquireSheinSubmitTask:          s.acquireSheinSubmitTask,
		shouldStartSheinPublishWorkflow: s.shouldStartSheinPublishWorkflow,
		submitSheinTaskWithWorkflow:     s.submitSheinTaskWithWorkflow,
		submitSheinTaskDirect:           direct.submitSheinTaskDirect,
		buildTaskPreview:                s.buildTaskPreview,
		buildSheinSubmitProductAPI:      execution.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:        s.buildSheinSubmitOtherAPI,
		mutateTaskResult:                recovery.mutateTaskResult,
		resolveRemoteStatus:             recovery.resolveSheinSubmitRemoteStatus,
	}
}

func buildTaskDirectSubmissionServiceConfig(s *service) taskDirectSubmissionServiceConfig {
	state := s.taskSubmissionStateOrDefault()
	execution := s.taskSubmissionExecutionOrDefault()
	return taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage:     execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:   s.validateSheinPublishFreshness,
		failSheinDirectSubmit:           state.failSheinDirectSubmit,
		buildSheinSubmitProductAPI:      execution.buildSheinSubmitProductAPI,
		persistSheinDirectSubmitPhase:   state.persistSheinDirectSubmitPhase,
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
	state := s.taskSubmissionStateOrDefault()
	execution := s.taskSubmissionExecutionOrDefault()
	recovery := s.taskSubmissionRecoveryOrDefault()
	return taskTemporalSubmissionAdapterConfig{
		beginSheinSubmitLease:                recovery.beginSheinSubmitLease,
		loadSheinPublishTask:                 s.loadSheinPublishTask,
		normalizeSheinSubmitPackage:          execution.normalizeSheinSubmitPackage,
		validateSheinPublishFreshness:        s.validateSheinPublishFreshness,
		saveTaskResult:                       s.repo.SaveTaskResult,
		persistSheinSubmitPhase:              state.persistSheinSubmitPhase,
		prepareSheinSubmitProduct:            execution.prepareSheinSubmitProduct,
		uploadSheinSubmitImages:              execution.uploadSheinSubmitImages,
		resolveSubmitSettings:                resolver.resolveSubmitSettings,
		buildSheinSubmitProductAPI:           execution.buildSheinSubmitProductAPI,
		preValidateSheinSubmitProduct:        execution.preValidateSheinSubmitProduct,
		executeSheinSubmitRemote:             execution.executeSheinSubmitRemote,
		retrySheinSensitiveWordSubmit:        s.retrySheinSensitiveWordSubmit,
		persistSuccessfulSheinSubmission:     state.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: state.recordSheinSubmissionFailureForState,
		refreshSheinSubmitRemoteStatus:       recovery.refreshSheinSubmitRemoteStatus,
		rememberSheinSubmitted:               s.rememberSheinSubmittedResolution,
		getTaskPreview:                       s.GetTaskPreview,
	}
}
