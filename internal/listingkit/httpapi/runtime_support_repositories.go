package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
)

func buildRuntimeSupportRepositories() BuildServiceRepositories {
	return BuildServiceRepositories{
		Core: CoreRepositoryBuilders{
			Task:                  BuildListingKitTaskRepository,
			StudioAsyncJob:        BuildListingKitStudioAsyncJobRepository,
			StudioBatch:           BuildListingKitStudioBatchRepository,
			StudioBatchRun:        BuildListingKitStudioBatchRunRepository,
			SheinSync:             BuildListingKitSheinSyncRepository,
			Subscription:          BuildListingSubscriptionRepository,
			MemberInvitationAudit: BuildMemberInvitationAuditRepository,
			ApprovedAsset:         BuildApprovedAssetInventoryReader,
			Review:                BuildListingKitReviewRepository,
			StudioSession:         BuildListingKitStudioSessionRepository,
			UploadedImage:         BuildListingKitUploadedImageRepository,
			StoreProfile:          BuildListingKitStoreProfileRepository,
			SheinResolutionCache:  BuildSheinResolutionCacheStore,
		},
		Admin: AdminRepositoryBuilders{
			Store:                   BuildListingAdminStoreRepository,
			StoreStatistics:         BuildListingAdminStoreStatisticsRepository,
			DispatchEvent:           BuildListingAdminDispatchEventRepository,
			ImportTask:              BuildListingAdminImportTaskRepository,
			FilterRule:              BuildListingAdminFilterRuleRepository,
			ProfitRule:              BuildListingAdminProfitRuleRepository,
			PricingRule:             BuildListingAdminPricingRuleRepository,
			OperationStrategy:       BuildListingAdminOperationStrategyRepository,
			ScheduledTaskConfig:     BuildListingAdminScheduledTaskConfigRepository,
			SensitiveWord:           BuildListingAdminSensitiveWordRepository,
			GenerationTopicOverride: BuildListingAdminGenerationTopicOverrideRepository,
			GenerationTopicPolicy:   BuildListingAdminGenerationTopicPolicyRepository,
			ProductImportMapping:    BuildListingAdminProductImportMappingRepository,
			Category:                BuildListingAdminCategoryRepository,
			ProductData:             BuildListingAdminProductDataRepository,
		},
	}
}

func withApprovedAssetReader(repositories BuildServiceRepositories, approvedAssets listingkit.ApprovedAssetInventoryReader) BuildServiceRepositories {
	if approvedAssets == nil {
		return repositories
	}
	repositories.Core.ApprovedAsset = func(*config.Config, *logrus.Logger) (listingkit.ApprovedAssetInventoryReader, []func() error, error) {
		return approvedAssets, nil, nil
	}
	return repositories
}
