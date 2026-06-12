package httpapi

import (
	"fmt"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

type builtRepositories struct {
	taskRepository                    listingkit.Repository
	studioAsyncJobRepository          listingkit.StudioAsyncJobRepository
	studioBatchRepository             listingkit.StudioBatchRepository
	studioBatchRunRepository          listingkit.StudioBatchRunRepository
	sheinSyncRepository               listingkit.SheinSyncRepository
	storeRepository                   listingadmin.StoreRepository
	storeStatisticsRepository         listingadmin.StoreStatisticsRepository
	importTaskRepository              listingadmin.ImportTaskRepository
	filterRuleRepository              listingadmin.FilterRuleRepository
	profitRuleRepository              listingadmin.ProfitRuleRepository
	pricingRuleRepository             listingadmin.PricingRuleRepository
	operationStrategyRepository       listingadmin.OperationStrategyRepository
	sensitiveWordRepository           listingadmin.SensitiveWordRepository
	generationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	generationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
	productImportMappingRepository    listingadmin.ProductImportMappingRepository
	categoryRepository                listingadmin.CategoryRepository
	productDataRepository             listingadmin.ProductDataRepository
	subscriptionService               *listingsubscription.Service
	assetRepository                   assetrepo.Repository
	reviewRepository                  reviewstore.Repository
	studioSessionRepository           listingkit.StudioSessionRepository
	uploadedImageRepository           listingkit.UploadedImageRepository
	storeProfileRepository            listingkit.StoreProfileRepository
	resolutionCacheStore              sheinpub.ResolutionCacheStore
}

type builtCoreRepositories struct {
	taskRepository           listingkit.Repository
	studioAsyncJobRepository listingkit.StudioAsyncJobRepository
	studioBatchRepository    listingkit.StudioBatchRepository
	studioBatchRunRepository listingkit.StudioBatchRunRepository
	sheinSyncRepository      listingkit.SheinSyncRepository
}

type coreTaskRepositories struct {
	taskRepository listingkit.Repository
}

type coreAsyncRepositories struct {
	studioAsyncJobRepository listingkit.StudioAsyncJobRepository
	studioBatchRepository    listingkit.StudioBatchRepository
	studioBatchRunRepository listingkit.StudioBatchRunRepository
	sheinSyncRepository      listingkit.SheinSyncRepository
}

type builtLateCoreRepositories struct {
	subscriptionService            *listingsubscription.Service
	assetRepository                assetrepo.Repository
	reviewRepository               reviewstore.Repository
	studioSessionRepository        listingkit.StudioSessionRepository
	uploadedImageRepository        listingkit.UploadedImageRepository
	storeProfileRepository         listingkit.StoreProfileRepository
	resolutionCacheStore           sheinpub.ResolutionCacheStore
}

type lateCoreRepositoryDependencies struct {
	assetRepository                assetrepo.Repository
	reviewRepository               reviewstore.Repository
	studioSessionRepository        listingkit.StudioSessionRepository
	uploadedImageRepository        listingkit.UploadedImageRepository
	storeProfileRepository         listingkit.StoreProfileRepository
	resolutionCacheStore           sheinpub.ResolutionCacheStore
}

type builtAdminRepositories struct {
	storeRepository                   listingadmin.StoreRepository
	storeStatisticsRepository         listingadmin.StoreStatisticsRepository
	importTaskRepository              listingadmin.ImportTaskRepository
	filterRuleRepository              listingadmin.FilterRuleRepository
	profitRuleRepository              listingadmin.ProfitRuleRepository
	pricingRuleRepository             listingadmin.PricingRuleRepository
	operationStrategyRepository       listingadmin.OperationStrategyRepository
	sensitiveWordRepository           listingadmin.SensitiveWordRepository
	generationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	generationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
	productImportMappingRepository    listingadmin.ProductImportMappingRepository
	categoryRepository                listingadmin.CategoryRepository
	productDataRepository             listingadmin.ProductDataRepository
}

type adminCatalogRepositories struct {
	storeRepository                listingadmin.StoreRepository
	storeStatisticsRepository      listingadmin.StoreStatisticsRepository
	importTaskRepository           listingadmin.ImportTaskRepository
	productImportMappingRepository listingadmin.ProductImportMappingRepository
	categoryRepository             listingadmin.CategoryRepository
	productDataRepository          listingadmin.ProductDataRepository
}

type adminRuleRepositories struct {
	filterRuleRepository              listingadmin.FilterRuleRepository
	profitRuleRepository              listingadmin.ProfitRuleRepository
	pricingRuleRepository             listingadmin.PricingRuleRepository
	operationStrategyRepository       listingadmin.OperationStrategyRepository
	sensitiveWordRepository           listingadmin.SensitiveWordRepository
	generationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	generationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
}

type repositoryAssembly struct {
	core     *builtCoreRepositories
	admin    *builtAdminRepositories
	lateCore *builtLateCoreRepositories
	merged   *builtRepositories
}

func buildCoreRepositories(input BuildServiceInput, closers *closerStack) (*builtCoreRepositories, error) {
	taskRepos, err := buildCoreTaskRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	asyncRepos, err := buildCoreAsyncRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return &builtCoreRepositories{
		taskRepository:           taskRepos.taskRepository,
		studioAsyncJobRepository: asyncRepos.studioAsyncJobRepository,
		studioBatchRepository:    asyncRepos.studioBatchRepository,
		studioBatchRunRepository: asyncRepos.studioBatchRunRepository,
		sheinSyncRepository:      asyncRepos.sheinSyncRepository,
	}, nil
}

func buildCoreTaskRepositories(input BuildServiceInput, closers *closerStack) (*coreTaskRepositories, error) {
	taskRepository, err := buildNamedWithClosers("core.task", input.Repositories.Core.Task, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	return &coreTaskRepositories{
		taskRepository: taskRepository,
	}, nil
}

func buildCoreAsyncRepositories(input BuildServiceInput, closers *closerStack) (*coreAsyncRepositories, error) {
	studioAsyncJobRepository, err := buildNamedWithClosers("core.studio_async_job", input.Repositories.Core.StudioAsyncJob, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioBatchRepository, err := buildNamedWithClosers("core.studio_batch", input.Repositories.Core.StudioBatch, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioBatchRunRepository, err := buildNamedWithClosers("core.studio_batch_run", input.Repositories.Core.StudioBatchRun, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	sheinSyncRepository, err := buildNamedWithClosers("core.shein_sync", input.Repositories.Core.SheinSync, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	return &coreAsyncRepositories{
		studioAsyncJobRepository: studioAsyncJobRepository,
		studioBatchRepository:    studioBatchRepository,
		studioBatchRunRepository: studioBatchRunRepository,
		sheinSyncRepository:      sheinSyncRepository,
	}, nil
}

func buildLateCoreRepositories(input BuildServiceInput, closers *closerStack) (*builtLateCoreRepositories, error) {
	subscriptionService, err := buildSubscriptionService(input, closers)
	if err != nil {
		return nil, err
	}
	dependencies, err := buildLateCoreRepositoryDependencies(input, closers)
	if err != nil {
		return nil, err
	}

	return &builtLateCoreRepositories{
		subscriptionService:            subscriptionService,
		assetRepository:                dependencies.assetRepository,
		reviewRepository:               dependencies.reviewRepository,
		studioSessionRepository:        dependencies.studioSessionRepository,
		uploadedImageRepository:        dependencies.uploadedImageRepository,
		storeProfileRepository:         dependencies.storeProfileRepository,
		resolutionCacheStore:           dependencies.resolutionCacheStore,
	}, nil
}

func buildSubscriptionService(input BuildServiceInput, closers *closerStack) (*listingsubscription.Service, error) {
	subscriptionRepository, err := buildNamedWithClosers("core.subscription", input.Repositories.Core.Subscription, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	subscriptionService, err := listingsubscription.NewService(subscriptionRepository)
	if err != nil {
		return nil, fmt.Errorf("create listing subscription service: %w", err)
	}
	return subscriptionService, nil
}

func buildLateCoreRepositoryDependencies(input BuildServiceInput, closers *closerStack) (*lateCoreRepositoryDependencies, error) {
	repoBuilders := input.Repositories.Core

	assetRepository, err := buildNamedWithClosers("core.asset", repoBuilders.Asset, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	reviewRepository, err := buildNamedWithClosers("core.review", repoBuilders.Review, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	studioSessionRepository, err := buildNamedWithClosers("core.studio_session", repoBuilders.StudioSession, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	uploadedImageRepository, err := buildNamedWithClosers("core.uploaded_image", repoBuilders.UploadedImage, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeProfileRepository, err := buildNamedWithClosers("core.store_profile", repoBuilders.StoreProfile, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	resolutionCacheStore, err := buildNamedWithClosers("core.shein_resolution_cache", repoBuilders.SheinResolutionCache, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &lateCoreRepositoryDependencies{
		assetRepository:                assetRepository,
		reviewRepository:               reviewRepository,
		studioSessionRepository:        studioSessionRepository,
		uploadedImageRepository:        uploadedImageRepository,
		storeProfileRepository:         storeProfileRepository,
		resolutionCacheStore:           resolutionCacheStore,
	}, nil
}

func buildAdminRepositories(input BuildServiceInput, closers *closerStack) (*builtAdminRepositories, error) {
	catalog, err := buildAdminCatalogRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	rules, err := buildAdminRuleRepositories(input, closers)
	if err != nil {
		return nil, err
	}
	return &builtAdminRepositories{
		storeRepository:                   catalog.storeRepository,
		storeStatisticsRepository:         catalog.storeStatisticsRepository,
		importTaskRepository:              catalog.importTaskRepository,
		filterRuleRepository:              rules.filterRuleRepository,
		profitRuleRepository:              rules.profitRuleRepository,
		pricingRuleRepository:             rules.pricingRuleRepository,
		operationStrategyRepository:       rules.operationStrategyRepository,
		sensitiveWordRepository:           rules.sensitiveWordRepository,
		generationTopicOverrideRepository: rules.generationTopicOverrideRepository,
		generationTopicPolicyRepository:   rules.generationTopicPolicyRepository,
		productImportMappingRepository:    catalog.productImportMappingRepository,
		categoryRepository:                catalog.categoryRepository,
		productDataRepository:             catalog.productDataRepository,
	}, nil
}

func buildAdminCatalogRepositories(input BuildServiceInput, closers *closerStack) (*adminCatalogRepositories, error) {
	repoBuilders := input.Repositories.Admin

	storeRepository, err := buildNamedWithClosers("admin.store", repoBuilders.Store, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	storeStatisticsRepository, err := buildNamedWithClosers("admin.store_statistics", repoBuilders.StoreStatistics, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	importTaskRepository, err := buildNamedWithClosers("admin.import_task", repoBuilders.ImportTask, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	productImportMappingRepository, err := buildNamedWithClosers("admin.product_import_mapping", repoBuilders.ProductImportMapping, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	categoryRepository, err := buildNamedWithClosers("admin.category", repoBuilders.Category, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	productDataRepository, err := buildNamedWithClosers("admin.product_data", repoBuilders.ProductData, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &adminCatalogRepositories{
		storeRepository:                storeRepository,
		storeStatisticsRepository:      storeStatisticsRepository,
		importTaskRepository:           importTaskRepository,
		productImportMappingRepository: productImportMappingRepository,
		categoryRepository:             categoryRepository,
		productDataRepository:          productDataRepository,
	}, nil
}

func buildAdminRuleRepositories(input BuildServiceInput, closers *closerStack) (*adminRuleRepositories, error) {
	repoBuilders := input.Repositories.Admin

	filterRuleRepository, err := buildNamedWithClosers("admin.filter_rule", repoBuilders.FilterRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	profitRuleRepository, err := buildNamedWithClosers("admin.profit_rule", repoBuilders.ProfitRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	pricingRuleRepository, err := buildNamedWithClosers("admin.pricing_rule", repoBuilders.PricingRule, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	operationStrategyRepository, err := buildNamedWithClosers("admin.operation_strategy", repoBuilders.OperationStrategy, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	sensitiveWordRepository, err := buildNamedWithClosers("admin.sensitive_word", repoBuilders.SensitiveWord, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	generationTopicOverrideRepository, err := buildNamedWithClosers("admin.generation_topic_override", repoBuilders.GenerationTopicOverride, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}
	generationTopicPolicyRepository, err := buildNamedWithClosers("admin.generation_topic_policy", repoBuilders.GenerationTopicPolicy, input.Config, input.Logger, closers)
	if err != nil {
		return nil, err
	}

	return &adminRuleRepositories{
		filterRuleRepository:              filterRuleRepository,
		profitRuleRepository:              profitRuleRepository,
		pricingRuleRepository:             pricingRuleRepository,
		operationStrategyRepository:       operationStrategyRepository,
		sensitiveWordRepository:           sensitiveWordRepository,
		generationTopicOverrideRepository: generationTopicOverrideRepository,
		generationTopicPolicyRepository:   generationTopicPolicyRepository,
	}, nil
}

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
