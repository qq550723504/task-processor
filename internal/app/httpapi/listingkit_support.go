package httpapi

import (
	"strings"

	"github.com/sirupsen/logrus"

	appruntime "task-processor/internal/app/runtime"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	sdsusecase "task-processor/internal/sds/usecase"
)

func newListingKitBuildModuleInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.BuildModuleInput {
	return listingkithttpapi.BuildModuleInput{
		ServiceInput:                       newListingKitBuildServiceInput(logger, deps),
		ShouldStartTemporalWorkerInProcess: appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess(),
	}
}

func newListingKitBuildServiceInput(logger *logrus.Logger, deps *runtimeDeps) listingkithttpapi.BuildServiceInput {
	return listingkithttpapi.BuildServiceInput{
		Config:                                deps.cfg,
		Logger:                                logger,
		ProductService:                        deps.productService,
		ImageService:                          deps.imageService,
		SDSSyncService:                        buildSDSSyncService(logger, deps),
		ImageSubjectExtractor:                 deps.imageSubjectExtractor,
		ImageWhiteBackgroundRender:            deps.imageWhiteBgRenderer,
		ImageSceneRenderer:                    deps.imageSceneRenderer,
		ManagementClient:                      deps.managementClient,
		AICredentialStore:                     deps.aiCredentialStore,
		TaskRepositoryBuilder:                 listingkithttpapi.BuildListingKitTaskRepository,
		StoreRepositoryBuilder:                listingkithttpapi.BuildListingAdminStoreRepository,
		StoreStatisticsRepositoryBuilder:      listingkithttpapi.BuildListingAdminStoreStatisticsRepository,
		ImportTaskRepositoryBuilder:           listingkithttpapi.BuildListingAdminImportTaskRepository,
		FilterRuleRepositoryBuilder:           listingkithttpapi.BuildListingAdminFilterRuleRepository,
		ProfitRuleRepositoryBuilder:           listingkithttpapi.BuildListingAdminProfitRuleRepository,
		PricingRuleRepositoryBuilder:          listingkithttpapi.BuildListingAdminPricingRuleRepository,
		OperationStrategyRepositoryBuilder:    listingkithttpapi.BuildListingAdminOperationStrategyRepository,
		SensitiveWordRepositoryBuilder:        listingkithttpapi.BuildListingAdminSensitiveWordRepository,
		ProductImportMappingRepositoryBuilder: listingkithttpapi.BuildListingAdminProductImportMappingRepository,
		CategoryRepositoryBuilder:             listingkithttpapi.BuildListingAdminCategoryRepository,
		ProductDataRepositoryBuilder:          listingkithttpapi.BuildListingAdminProductDataRepository,
		SubscriptionRepositoryBuilder:         listingkithttpapi.BuildListingSubscriptionRepository,
		AssetRepositoryBuilder:                listingkithttpapi.BuildAssetRepository,
		ReviewRepositoryBuilder:               listingkithttpapi.BuildListingKitReviewRepository,
		StudioSessionRepositoryBuilder:        listingkithttpapi.BuildListingKitStudioSessionRepository,
		UploadedImageRepositoryBuilder:        listingkithttpapi.BuildListingKitUploadedImageRepository,
		StoreProfileRepositoryBuilder:         listingkithttpapi.BuildListingKitStoreProfileRepository,
		StoreRoutingSettingsRepositoryBuilder: listingkithttpapi.BuildListingKitStoreRoutingSettingsRepository,
		ResolutionCacheStoreBuilder:           listingkithttpapi.BuildSheinResolutionCacheStore,
		SheinPricingPolicyBuilder:             listingkithttpapi.BuildSheinPricingPolicy,
		ImageUploadStoreBuilder:               listingkithttpapi.BuildImageUploadStore,
		LegacyTenantResolverConfigurator:      listingkithttpapi.ConfigureLegacyTenantResolver,
		SheinCategoryLLMClientBuilder:         listingkithttpapi.BuildSheinCategoryLLMClient,
		SheinSaleAttributeLLMBuilder:          listingkithttpapi.BuildSheinSaleAttributeLLMClient,
		StudioImageGeneratorBuilder:           listingkithttpapi.BuildStudioImageGenerator,
		DefaultSheinStoreIDResolver:           listingkithttpapi.ResolveDefaultSheinStoreID,
		ConfigureZitadelAuth:                  listingkithttpapi.ConfigureListingKitZitadelAuth,
		ConfigureAuthorization:                listingkithttpapi.ConfigureListingKitAuthorization,
	}
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
