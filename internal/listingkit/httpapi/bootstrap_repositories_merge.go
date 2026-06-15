package httpapi

func applyCoreRepositories(repos *builtRepositories, core *builtCoreRepositories) {
	if repos == nil || core == nil {
		return
	}
	repos.taskRepository = core.taskRepository
	repos.studioAsyncJobRepository = core.studioAsyncJobRepository
	repos.studioBatchRepository = core.studioBatchRepository
	repos.studioBatchRunRepository = core.studioBatchRunRepository
	repos.sheinSyncRepository = core.sheinSyncRepository
}

func applyLateCoreRepositories(repos *builtRepositories, lateCore *builtLateCoreRepositories) {
	if repos == nil || lateCore == nil {
		return
	}
	repos.subscriptionService = lateCore.subscriptionService
	repos.assetRepository = lateCore.assetRepository
	repos.reviewRepository = lateCore.reviewRepository
	repos.studioSessionRepository = lateCore.studioSessionRepository
	repos.uploadedImageRepository = lateCore.uploadedImageRepository
	repos.storeProfileRepository = lateCore.storeProfileRepository
	repos.resolutionCacheStore = lateCore.resolutionCacheStore
}

func applyAdminRepositories(repos *builtRepositories, admin *builtAdminRepositories) {
	if repos == nil || admin == nil {
		return
	}
	repos.storeRepository = admin.storeRepository
	repos.storeStatisticsRepository = admin.storeStatisticsRepository
	repos.importTaskRepository = admin.importTaskRepository
	repos.filterRuleRepository = admin.filterRuleRepository
	repos.profitRuleRepository = admin.profitRuleRepository
	repos.pricingRuleRepository = admin.pricingRuleRepository
	repos.operationStrategyRepository = admin.operationStrategyRepository
	repos.sensitiveWordRepository = admin.sensitiveWordRepository
	repos.generationTopicOverrideRepository = admin.generationTopicOverrideRepository
	repos.generationTopicPolicyRepository = admin.generationTopicPolicyRepository
	repos.productImportMappingRepository = admin.productImportMappingRepository
	repos.categoryRepository = admin.categoryRepository
	repos.productDataRepository = admin.productDataRepository
}

func mergeBuiltRepositories(core *builtCoreRepositories, lateCore *builtLateCoreRepositories, admin *builtAdminRepositories) *builtRepositories {
	repos := &builtRepositories{}
	applyCoreRepositories(repos, core)
	applyLateCoreRepositories(repos, lateCore)
	applyAdminRepositories(repos, admin)
	return repos
}

func assembleRepositories(input BuildServiceInput, closers *closerStack) (*repositoryAssembly, error) {
	coreRepos, err := buildCoreRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	adminRepos, err := buildAdminRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	lateCoreRepos, err := buildLateCoreRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return &repositoryAssembly{
		core:     coreRepos,
		admin:    adminRepos,
		lateCore: lateCoreRepos,
		merged:   mergeBuiltRepositories(coreRepos, lateCoreRepos, adminRepos),
	}, nil
}

func buildRepositories(input BuildServiceInput, closers *closerStack) (*builtRepositories, error) {
	assembly, err := assembleRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return assembly.merged, nil
}
