package httpapi

import (
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/memberinvite"
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
	dispatchEventRepository           listingadmin.DispatchEventRepository
	importTaskRepository              listingadmin.ImportTaskRepository
	filterRuleRepository              listingadmin.FilterRuleRepository
	profitRuleRepository              listingadmin.ProfitRuleRepository
	pricingRuleRepository             listingadmin.PricingRuleRepository
	operationStrategyRepository       listingadmin.OperationStrategyRepository
	scheduledTaskConfigRepository     listingadmin.ScheduledTaskConfigRepository
	sensitiveWordRepository           listingadmin.SensitiveWordRepository
	generationTopicOverrideRepository listingadmin.GenerationTopicOverrideRepository
	generationTopicPolicyRepository   listingadmin.GenerationTopicPolicyRepository
	productImportMappingRepository    listingadmin.ProductImportMappingRepository
	categoryRepository                listingadmin.CategoryRepository
	productDataRepository             listingadmin.ProductDataRepository
	subscriptionService               *listingsubscription.Service
	memberInvitationAuditRepository   memberinvite.AuditRepository
	approvedAssetInventoryReader      listingkit.ApprovedAssetInventoryReader
	reviewRepository                  reviewstore.Repository
	studioSessionRepository           listingkit.StudioSessionRepository
	uploadedImageRepository           listingkit.UploadedImageRepository
	storeProfileRepository            listingkit.StoreProfileRepository
	resolutionCacheStore              sheinpub.ResolutionCacheStore
}
