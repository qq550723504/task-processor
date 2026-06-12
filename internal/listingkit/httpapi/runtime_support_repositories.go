package httpapi

func buildRuntimeSupportRepositories() BuildServiceRepositories {
	return BuildServiceRepositories{
		Core: CoreRepositoryBuilders{
			Task:                 BuildListingKitTaskRepository,
			StudioAsyncJob:       BuildListingKitStudioAsyncJobRepository,
			StudioBatch:          BuildListingKitStudioBatchRepository,
			StudioBatchRun:       BuildListingKitStudioBatchRunRepository,
			SheinSync:            BuildListingKitSheinSyncRepository,
			Subscription:         BuildListingSubscriptionRepository,
			Asset:                BuildAssetRepository,
			Review:               BuildListingKitReviewRepository,
			StudioSession:        BuildListingKitStudioSessionRepository,
			UploadedImage:        BuildListingKitUploadedImageRepository,
			StoreProfile:         BuildListingKitStoreProfileRepository,
			LegacyStoreRoutingSettings: BuildListingKitStoreRoutingSettingsRepository,
			SheinResolutionCache: BuildSheinResolutionCacheStore,
		},
		Admin: AdminRepositoryBuilders{
			Store:                   BuildListingAdminStoreRepository,
			StoreStatistics:         BuildListingAdminStoreStatisticsRepository,
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
