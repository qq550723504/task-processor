package httpapi

import (
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sheinlogin"
)

type RuntimeSupportInput struct {
	SheinCookieStore          *sheinlogin.RedisStore
	SDSSyncService            sdsusecase.Service
	SDSLoginStatusProvider    listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
}

type RuntimeSupport struct {
	Repositories              BuildServiceRepositories
	Hooks                     BuildServiceHooks
	SDSSyncService            sdsusecase.Service
	SDSLoginStatusProvider    listingkit.SDSLoginStatusProvider
	SDSBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
}

func BuildRuntimeSupport(input RuntimeSupportInput) RuntimeSupport {
	return RuntimeSupport{
		Repositories:              buildRuntimeSupportRepositories(),
		Hooks:                     buildRuntimeSupportHooks(input.SheinCookieStore),
		SDSSyncService:            input.SDSSyncService,
		SDSLoginStatusProvider:    input.SDSLoginStatusProvider,
		SDSBaselineRemoteProvider: input.SDSBaselineRemoteProvider,
	}
}

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
			StoreRoutingSettings: BuildListingKitStoreRoutingSettingsRepository,
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

func buildRuntimeSupportHooks(cookieStore *sheinlogin.RedisStore) BuildServiceHooks {
	return BuildServiceHooks{
		SheinPricingPolicyBuilder:        BuildSheinPricingPolicy,
		ImageUploadStoreBuilder:          BuildImageUploadStore,
		LegacyTenantResolverConfigurator: ConfigureLegacyTenantResolver,
		SheinCategoryLLMClientBuilder:    BuildSheinCategoryLLMClient,
		SheinSaleAttributeLLMBuilder:     BuildSheinSaleAttributeLLMClient,
		SheinCategoryResolverBuilder: func(storeRepo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
			return buildListingKitSheinCategoryResolver(storeRepo, cookieStore, llm, cache)
		},
		SheinAttributeResolverBuilder: func(storeRepo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
			return buildListingKitSheinAttributeResolver(storeRepo, cookieStore, llm, cache)
		},
		SheinSaleAttributeResolverBuilder: func(storeRepo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
			return buildListingKitSheinSaleAttributeResolver(storeRepo, cookieStore, llm, cache)
		},
		SheinProductAPIBuilderFactory: func(storeRepo listingadmin.StoreRepository) sheinpub.ProductAPIBuilder {
			return buildListingKitSheinProductAPIBuilder(storeRepo, cookieStore)
		},
		SheinImageAPIBuilderFactory: func(storeRepo listingadmin.StoreRepository) sheinpub.ImageAPIBuilder {
			return buildListingKitSheinImageAPIBuilder(storeRepo, cookieStore)
		},
		SheinTranslateAPIBuilderFactory: func(storeRepo listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder {
			return buildListingKitSheinTranslateAPIBuilder(storeRepo, cookieStore)
		},
		SheinAPIClientFactoryBuilder: func(storeRepo listingadmin.StoreRepository) listingkit.SheinAPIClientFactory {
			return listingKitSheinAPIClientFactory{repo: storeRepo, cookieStore: cookieStore}
		},
		StudioImageGeneratorBuilder: BuildStudioImageGenerator,
		DefaultSheinStoreIDResolver: ResolveDefaultSheinStoreID,
		ConfigureZitadelAuth:        ConfigureListingKitZitadelAuth,
		ConfigureAuthorization:      ConfigureListingKitAuthorization,
	}
}
