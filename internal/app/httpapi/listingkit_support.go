package httpapi

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	appruntime "task-processor/internal/app/runtime"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	listingadmin "task-processor/internal/listingadmin"
	listingkit "task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/tenantbridge"
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
		TaskRepositoryBuilder:                 buildListingKitTaskRepository,
		StoreRepositoryBuilder:                buildListingAdminStoreRepository,
		StoreStatisticsRepositoryBuilder:      buildListingAdminStoreStatisticsRepository,
		ImportTaskRepositoryBuilder:           buildListingAdminImportTaskRepository,
		FilterRuleRepositoryBuilder:           buildListingAdminFilterRuleRepository,
		ProfitRuleRepositoryBuilder:           buildListingAdminProfitRuleRepository,
		PricingRuleRepositoryBuilder:          buildListingAdminPricingRuleRepository,
		OperationStrategyRepositoryBuilder:    buildListingAdminOperationStrategyRepository,
		SensitiveWordRepositoryBuilder:        buildListingAdminSensitiveWordRepository,
		ProductImportMappingRepositoryBuilder: buildListingAdminProductImportMappingRepository,
		CategoryRepositoryBuilder:             buildListingAdminCategoryRepository,
		ProductDataRepositoryBuilder:          buildListingAdminProductDataRepository,
		SubscriptionRepositoryBuilder:         buildListingSubscriptionRepository,
		AssetRepositoryBuilder:                buildAssetRepository,
		ReviewRepositoryBuilder:               buildListingKitReviewRepository,
		StudioSessionRepositoryBuilder:        buildListingKitStudioSessionRepository,
		UploadedImageRepositoryBuilder:        buildListingKitUploadedImageRepository,
		StoreProfileRepositoryBuilder:         buildListingKitStoreProfileRepository,
		StoreRoutingSettingsRepositoryBuilder: buildListingKitStoreRoutingSettingsRepository,
		ResolutionCacheStoreBuilder:           buildSheinResolutionCacheStore,
		SheinPricingPolicyBuilder:             buildListingKitSheinPricingPolicy,
		ImageUploadStoreBuilder:               buildListingKitImageUploadStore,
		LegacyTenantResolverConfigurator:      configureListingKitLegacyTenantResolver,
		SheinCategoryLLMClientBuilder:         listingkithttpapi.BuildSheinCategoryLLMClient,
		SheinSaleAttributeLLMBuilder:          listingkithttpapi.BuildSheinSaleAttributeLLMClient,
		StudioImageGeneratorBuilder:           listingkithttpapi.BuildStudioImageGenerator,
		DefaultSheinStoreIDResolver:           listingkithttpapi.ResolveDefaultSheinStoreID,
		ConfigureZitadelAuth:                  listingkithttpapi.ConfigureListingKitZitadelAuth,
		ConfigureAuthorization:                listingkithttpapi.ConfigureListingKitAuthorization,
	}
}

func buildListingAdminStoreRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminStoreRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin store repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit store admin API disabled")
	return nil, nil, nil
}

func buildListingKitStoreProfileRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStoreProfileRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit store profile repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit store profile repository")
	return nil, nil, nil
}

func buildListingKitStoreRoutingSettingsRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStoreRoutingSettingsRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit store routing repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit store routing repository")
	return nil, nil, nil
}

func buildListingAdminStoreStatisticsRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminStoreStatisticsRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin store statistics repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit store statistics admin API disabled")
	return nil, nil, nil
}

func buildListingAdminImportTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminImportTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin import task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit import task admin API disabled")
	return nil, nil, nil
}

func buildListingAdminFilterRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminFilterRuleRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin filter rule repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit filter rule admin API disabled")
	return nil, nil, nil
}

func buildListingAdminProfitRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminProfitRuleRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin profit rule repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit profit rule admin API disabled")
	return nil, nil, nil
}

func buildListingAdminPricingRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminPricingRuleRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin pricing rule repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit pricing rule admin API disabled")
	return nil, nil, nil
}

func buildListingAdminOperationStrategyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminOperationStrategyRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin operation strategy repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit operation strategy admin API disabled")
	return nil, nil, nil
}

func buildListingAdminSensitiveWordRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminSensitiveWordRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin sensitive word repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit sensitive word admin API disabled")
	return nil, nil, nil
}

func buildListingAdminProductImportMappingRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminProductImportMappingRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin product import mapping repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit product import mapping admin API disabled")
	return nil, nil, nil
}

func buildListingAdminCategoryRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminCategoryRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin category repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit category admin API disabled")
	return nil, nil, nil
}

func buildListingAdminProductDataRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminProductDataRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin product data repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit product data admin API disabled")
	return nil, nil, nil
}

func buildListingSubscriptionRepository(cfg *config.Config, logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingSubscriptionRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing subscription repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory ListingKit subscription repository")
	return listingsubscription.NewMemRepository(), nil, nil
}

func buildListingKitSheinPricingPolicy(cfg *config.Config) sheinpub.PricingPolicy {
	if cfg == nil {
		return sheinpub.PricingPolicy{}
	}
	pricing := cfg.Platforms.Shein.ListingPricing
	return sheinpub.PricingPolicy{
		Enabled:        pricing.Enabled,
		Currency:       pricing.Currency,
		MarkupRate:     pricing.MarkupRate,
		FixedMarkup:    pricing.FixedMarkup,
		ShippingCost:   pricing.ShippingCost,
		CommissionRate: pricing.CommissionRate,
		MinimumPrice:   pricing.MinimumPrice,
		RoundTo:        pricing.RoundTo,
	}
}

func buildListingKitImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	if cfg == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(cfg.ProductImage.Publisher.Provider), "s3") {
		return buildListingKitS3ImageUploadStore(cfg, logger)
	}
	rootDir := filepath.Join(cfg.ProductImage.Publisher.OutputDir, "listingkit-inputs")
	store, err := listingkit.NewLocalImageUploadStore(rootDir)
	if err != nil {
		return nil
	}
	return store
}

func configureListingKitLegacyTenantResolver(cfg *config.Config, logger *logrus.Logger) (func() error, error) {
	if cfg == nil || cfg.Database == nil || strings.TrimSpace(cfg.Database.Host) == "" {
		tenantbridge.ConfigureLegacyTenantResolver(nil)
		return nil, nil
	}
	for _, databaseName := range []string{"zitadel_auth", "zitadel"} {
		zitadelCfg := *cfg.Database
		zitadelCfg.Database = databaseName
		db, err := database.NewSharedDatabaseFromConfig(&zitadelCfg)
		if err != nil {
			continue
		}
		if !listingKitLegacyTenantMetadataTableExists(db) {
			_ = database.CloseSharedDatabase(&zitadelCfg, db)
			continue
		}
		tenantbridge.ConfigureLegacyTenantResolver(tenantbridge.NewMetadataResolver(db))
		logger.Infof("listingkit legacy tenant resolver connected: %s:%d/%s", zitadelCfg.Host, zitadelCfg.Port, zitadelCfg.Database)
		return func() error { return database.CloseSharedDatabase(&zitadelCfg, db) }, nil
	}
	tenantbridge.ConfigureLegacyTenantResolver(nil)
	logger.Warn("listingkit legacy tenant resolver metadata table not found; legacy tenant bridge disabled")
	return nil, nil
}

func listingKitLegacyTenantMetadataTableExists(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	result := struct {
		Name *string `gorm:"column:name"`
	}{}
	if err := db.Raw("select to_regclass(?) as name", "projections.org_metadata2").Scan(&result).Error; err != nil {
		return false
	}
	return result.Name != nil && strings.TrimSpace(*result.Name) != ""
}

func buildListingKitReviewRepository(cfg *config.Config, logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitReviewRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit review repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit review repository")
	return reviewstore.NewMemRepository(), nil, nil
}

func buildListingKitStudioSessionRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStudioSessionRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit studio session repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, SHEIN studio session repository disabled")
	return nil, nil, nil
}

func buildListingKitUploadedImageRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitUploadedImageRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit uploaded image repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit uploaded image repository")
	return listingkit.NewMemUploadedImageRepository(), nil, nil
}

func buildSheinResolutionCacheStore(cfg *config.Config, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		store, closer, err := newDBSheinResolutionCacheStore(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create shein resolution cache store: %w", err)
		}
		return store, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory SHEIN resolution cache fallback")
	return nil, nil, nil
}

func buildAssetRepository(cfg *config.Config, logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBAssetRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create asset repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory asset repository")
	return assetrepo.NewMemRepository(), nil, nil
}

func buildListingKitTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit repository")
	return listingkitstore.NewMemTaskRepository(), nil, nil
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
