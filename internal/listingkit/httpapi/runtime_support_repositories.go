package httpapi

func buildRuntimeSupportRepositories() BuildServiceRepositories {
	return BuildServiceRepositories{
		Core: CoreRepositoryBuilders{
			Task:                 BuildListingKitTaskRepository,
			StudioAsyncJob:       BuildListingKitStudioAsyncJobRepository,
			StudioBatch:          BuildListingKitStudioBatchRepository,
			StudioBatchRun:       BuildListingKitStudioBatchRunRepository,
			StudioBatchTaskLink:  BuildListingKitStudioBatchTaskLinkRepository,
			SheinSync:            BuildListingKitSheinSyncRepository,
			Subscription:         BuildListingSubscriptionRepository,
			Asset:                BuildAssetRepository,
			Review:               BuildListingKitReviewRepository,
			StudioSession:        BuildListingKitStudioSessionRepository,
			UploadedImage:        BuildListingKitUploadedImageRepository,
			StoreProfile:         BuildListingKitStoreProfileRepository,
			SheinResolutionCache: BuildSheinResolutionCacheStore,
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
			SensitiveWord:           BuildListingAdminSensitiveWordRepository,
			GenerationTopicOverride: BuildListingAdminGenerationTopicOverrideRepository,
			GenerationTopicPolicy:   BuildListingAdminGenerationTopicPolicyRepository,
			ProductImportMapping:    BuildListingAdminProductImportMappingRepository,
			Category:                BuildListingAdminCategoryRepository,
			ProductData:             BuildListingAdminProductDataRepository,
		},
	}
}
