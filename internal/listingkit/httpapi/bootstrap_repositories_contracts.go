package httpapi

import (
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
	studioBatchTaskLinkRepository     listingkit.StudioBatchTaskLinkRepository
	sheinSyncRepository               listingkit.SheinSyncRepository
	storeRepository                   listingadmin.StoreRepository
	storeStatisticsRepository         listingadmin.StoreStatisticsRepository
	dispatchEventRepository           listingadmin.DispatchEventRepository
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
	taskRepository                listingkit.Repository
	studioAsyncJobRepository      listingkit.StudioAsyncJobRepository
	studioBatchRepository         listingkit.StudioBatchRepository
	studioBatchRunRepository      listingkit.StudioBatchRunRepository
	studioBatchTaskLinkRepository listingkit.StudioBatchTaskLinkRepository
	sheinSyncRepository           listingkit.SheinSyncRepository
}

type coreTaskRepositories struct {
	taskRepository listingkit.Repository
}

type coreAsyncRepositories struct {
	studioAsyncJobRepository      listingkit.StudioAsyncJobRepository
	studioBatchRepository         listingkit.StudioBatchRepository
	studioBatchRunRepository      listingkit.StudioBatchRunRepository
	studioBatchTaskLinkRepository listingkit.StudioBatchTaskLinkRepository
	sheinSyncRepository           listingkit.SheinSyncRepository
}

type builtLateCoreRepositories struct {
	subscriptionService     *listingsubscription.Service
	assetRepository         assetrepo.Repository
	reviewRepository        reviewstore.Repository
	studioSessionRepository listingkit.StudioSessionRepository
	uploadedImageRepository listingkit.UploadedImageRepository
	storeProfileRepository  listingkit.StoreProfileRepository
	resolutionCacheStore    sheinpub.ResolutionCacheStore
}

type lateCoreRepositoryDependencies struct {
	assetRepository         assetrepo.Repository
	reviewRepository        reviewstore.Repository
	studioSessionRepository listingkit.StudioSessionRepository
	uploadedImageRepository listingkit.UploadedImageRepository
	storeProfileRepository  listingkit.StoreProfileRepository
	resolutionCacheStore    sheinpub.ResolutionCacheStore
}

type builtAdminRepositories struct {
	storeRepository                   listingadmin.StoreRepository
	storeStatisticsRepository         listingadmin.StoreStatisticsRepository
	dispatchEventRepository           listingadmin.DispatchEventRepository
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
	dispatchEventRepository        listingadmin.DispatchEventRepository
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
