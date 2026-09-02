package httpapi

import (
	"fmt"

	"task-processor/internal/listingsubscription"
)

func buildRepositories(input BuildServiceInput) (*builtRepositories, error) {
	values := input.Repositories
	subscriptionService, err := listingsubscription.NewServiceWithLedger(
		values.Core.Subscription,
		values.Core.GenerationUsageLedger,
	)
	if err != nil {
		return nil, fmt.Errorf("create listing subscription service with usage ledger: %w", err)
	}
	return &builtRepositories{
		taskRepository:                    values.Core.Task,
		studioAsyncJobRepository:          values.Core.StudioAsyncJob,
		studioBatchRepository:             values.Core.StudioBatch,
		studioBatchRunRepository:          values.Core.StudioBatchRun,
		sheinSyncRepository:               values.Core.SheinSync,
		storeRepository:                   values.Admin.Store,
		storeStatisticsRepository:         values.Admin.StoreStatistics,
		dispatchEventRepository:           values.Admin.DispatchEvent,
		importTaskRepository:              values.Admin.ImportTask,
		filterRuleRepository:              values.Admin.FilterRule,
		profitRuleRepository:              values.Admin.ProfitRule,
		pricingRuleRepository:             values.Admin.PricingRule,
		operationStrategyRepository:       values.Admin.OperationStrategy,
		scheduledTaskConfigRepository:     values.Admin.ScheduledTaskConfig,
		sensitiveWordRepository:           values.Admin.SensitiveWord,
		generationTopicOverrideRepository: values.Admin.GenerationTopicOverride,
		generationTopicPolicyRepository:   values.Admin.GenerationTopicPolicy,
		productImportMappingRepository:    values.Admin.ProductImportMapping,
		categoryRepository:                values.Admin.Category,
		productDataRepository:             values.Admin.ProductData,
		subscriptionService:               subscriptionService,
		memberInvitationAuditRepository:   values.Core.MemberInvitationAudit,
		approvedAssetInventoryReader:      values.Core.ApprovedAsset,
		reviewRepository:                  values.Core.Review,
		studioSessionRepository:           values.Core.StudioSession,
		uploadedImageRepository:           values.Core.UploadedImage,
		storeProfileRepository:            values.Core.StoreProfile,
		resolutionCacheStore:              values.Core.SheinResolutionCache,
	}, nil
}
