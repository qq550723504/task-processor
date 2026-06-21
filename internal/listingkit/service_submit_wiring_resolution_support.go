package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func resolveSubmissionStoreProfileRepo(s *service) StoreProfileRepository {
	if s == nil {
		return nil
	}
	return s.submissionDeps.storeProfileRepo
}

func resolveSubmissionStoreCatalog(s *service) SheinStoreCatalog {
	return resolveSheinStoreCatalog(s)
}

func resolveSubmissionAPIClientFactory(s *service) SheinAPIClientFactory {
	return resolveSheinAPIClientFactory(s)
}

func resolveSubmissionProductAPIBuilder(s *service) sheinpub.ProductAPIBuilder {
	if s == nil {
		return nil
	}
	return s.submissionDeps.sheinProductAPIBuilder
}

func resolveSubmissionImageAPIBuilder(s *service) sheinpub.ImageAPIBuilder {
	if s == nil {
		return nil
	}
	return s.submissionDeps.sheinImageAPIBuilder
}

func resolveSubmissionTranslateAPIBuilder(s *service) sheinpub.TranslateAPIBuilder {
	if s == nil {
		return nil
	}
	return s.submissionDeps.sheinTranslateAPIBuilder
}

func resolveSubmissionContentOptimizer(s *service) AIChatCompleter {
	return resolveSheinContentOptimizer(s)
}

func resolveSubmissionWorkflowClient(s *service) (SheinPublishWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return s.submissionDeps.sheinPublishWorkflowClient, s.submissionDeps.sheinPublishWorkflowEnabled
}

func buildTaskRequeueServiceConfigWithWiring(wiring taskSubmitterWiring) taskRequeueServiceConfig {
	return taskRequeueServiceConfig{
		repo:             wiring.repo,
		taskSubmitter:    wiring.taskSubmitter,
		standardWorkflow: wiring.standardWorkflow,
	}
}

func buildTaskRecoveryServiceConfigWithWiring(wiring taskSubmitterWiring) taskRecoveryServiceConfig {
	return taskRecoveryServiceConfig{
		repo:          wiring.repo,
		taskSubmitter: wiring.taskSubmitter,
	}
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

func buildTaskSubmissionStateServiceConfigWithSupport(wiring taskSubmissionSupportWiring) taskSubmissionStateServiceConfig {
	return taskSubmissionStateServiceConfig{
		repo:                   wiring.repo,
		rememberSheinSubmitted: wiring.rememberSheinSubmitted,
	}
}
