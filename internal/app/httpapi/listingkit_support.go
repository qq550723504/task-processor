package httpapi

import (
	"strings"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sheinlogin"
)

func newListingKitBuildModuleInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.BuildModuleInput {
	return listingkithttpapi.BuildModuleInput{
		ServiceInput:                       newListingKitBuildServiceInput(logger, deps),
		ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
	}
}

func newListingKitBuildServiceInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.BuildServiceInput {
	cookieStore := ensureListingKitSheinCookieStore(logger, deps)
	return listingkithttpapi.BuildServiceInput{
		Config:                     deps.cfg,
		Logger:                     logger,
		ProductService:             deps.productService,
		ImageService:               deps.imageService,
		SDSSyncService:             buildSDSSyncService(logger, deps),
		ImageSubjectExtractor:      deps.imageSubjectExtractor,
		ImageWhiteBackgroundRender: deps.imageWhiteBgRenderer,
		ImageSceneRenderer:         deps.imageSceneRenderer,
		AICredentialStore:          deps.aiCredentialStore,
		Repositories: listingkithttpapi.BuildServiceRepositories{
			Core: listingkithttpapi.CoreRepositoryBuilders{
				Task:                 listingkithttpapi.BuildListingKitTaskRepository,
				StudioAsyncJob:       listingkithttpapi.BuildListingKitStudioAsyncJobRepository,
				Subscription:         listingkithttpapi.BuildListingSubscriptionRepository,
				Asset:                listingkithttpapi.BuildAssetRepository,
				Review:               listingkithttpapi.BuildListingKitReviewRepository,
				StudioSession:        listingkithttpapi.BuildListingKitStudioSessionRepository,
				UploadedImage:        listingkithttpapi.BuildListingKitUploadedImageRepository,
				StoreProfile:         listingkithttpapi.BuildListingKitStoreProfileRepository,
				StoreRoutingSettings: listingkithttpapi.BuildListingKitStoreRoutingSettingsRepository,
				SheinResolutionCache: listingkithttpapi.BuildSheinResolutionCacheStore,
			},
			Admin: listingkithttpapi.AdminRepositoryBuilders{
				Store:                listingkithttpapi.BuildListingAdminStoreRepository,
				StoreStatistics:      listingkithttpapi.BuildListingAdminStoreStatisticsRepository,
				ImportTask:           listingkithttpapi.BuildListingAdminImportTaskRepository,
				FilterRule:           listingkithttpapi.BuildListingAdminFilterRuleRepository,
				ProfitRule:           listingkithttpapi.BuildListingAdminProfitRuleRepository,
				PricingRule:          listingkithttpapi.BuildListingAdminPricingRuleRepository,
				OperationStrategy:    listingkithttpapi.BuildListingAdminOperationStrategyRepository,
				SensitiveWord:        listingkithttpapi.BuildListingAdminSensitiveWordRepository,
				ProductImportMapping: listingkithttpapi.BuildListingAdminProductImportMappingRepository,
				Category:             listingkithttpapi.BuildListingAdminCategoryRepository,
				ProductData:          listingkithttpapi.BuildListingAdminProductDataRepository,
			},
		},
		Hooks: listingkithttpapi.BuildServiceHooks{
			SheinPricingPolicyBuilder:        listingkithttpapi.BuildSheinPricingPolicy,
			ImageUploadStoreBuilder:          listingkithttpapi.BuildImageUploadStore,
			LegacyTenantResolverConfigurator: listingkithttpapi.ConfigureLegacyTenantResolver,
			SheinCategoryLLMClientBuilder:    listingkithttpapi.BuildSheinCategoryLLMClient,
			SheinSaleAttributeLLMBuilder:     listingkithttpapi.BuildSheinSaleAttributeLLMClient,
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
			StudioImageGeneratorBuilder: listingkithttpapi.BuildStudioImageGenerator,
			DefaultSheinStoreIDResolver: listingkithttpapi.ResolveDefaultSheinStoreID,
			ConfigureZitadelAuth:        listingkithttpapi.ConfigureListingKitZitadelAuth,
			ConfigureAuthorization:      listingkithttpapi.ConfigureListingKitAuthorization,
		},
	}
}

func ensureListingKitSheinCookieStore(logger *logrus.Logger, deps *runtimeDeps) *sheinlogin.RedisStore {
	if deps == nil || deps.cfg == nil {
		return nil
	}
	if deps.listingKitSheinCookieStore != nil {
		return deps.listingKitSheinCookieStore
	}
	redisCfg := deps.cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(redisCfg.Host) == "" {
		return nil
	}
	store, err := sheinlogin.NewRedisStore(redisCfg)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("failed to initialize listingkit shein cookie store; shein runtime will degrade")
		}
		return nil
	}
	deps.listingKitSheinCookieStore = store
	deps.closers = append(deps.closers, store.Close)
	return store
}

func buildSDSSyncService(logger *logrus.Logger, deps *runtimeDeps) sdsusecase.Service {
	if deps == nil || deps.imageService == nil {
		return nil
	}

	svc, authState, err := newSDSSyncServiceForHTTPAPI(deps.imageService, buildSDSClientConfig(deps.cfg))
	if err != nil {
		logger.WithError(err).Warn("failed to initialize SDS client; SDS sync disabled")
		return nil
	}
	if svc == nil {
		logger.Warn("SDS sync service not initialized; SDS sync disabled")
		return nil
	}

	if authState == nil || strings.TrimSpace(authState.AccessToken) == "" {
		logger.Info("SDS auth state not found at startup; keeping SDS sync enabled for request-time auth bootstrap")
	}

	return svc
}
