package httpapi

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	storageinfra "task-processor/internal/infra/storage"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingkit/studiostore"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/tenantbridge"
)

func buildRepositoryWithFallback[T any](
	cfg *config.Config,
	logger *logrus.Logger,
	buildDB func(*config.DatabaseConfig, *logrus.Logger) (T, func() error, error),
	buildFallback func(*logrus.Logger) (T, []func() error, error),
) (T, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := buildDB(cfg.Database, logger)
		if err != nil {
			var zero T
			return zero, nil, err
		}
		return repo, []func() error{closer}, nil
	}
	return buildFallback(logger)
}

func BuildListingKitTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitTaskRepository, func(logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit repository")
		return listingkitstore.NewMemTaskRepository(), nil, nil
	})
}

func BuildListingKitStudioAsyncJobRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioAsyncJobRepository, func(logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio async job repository")
		return listingkit.NewMemStudioAsyncJobRepository(), nil, nil
	})
}

func BuildListingKitStudioBatchRunRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioBatchRunRepository, func(logger *logrus.Logger) (listingkit.StudioBatchRunRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio batch run repository for studio batch run APIs")
		return listingkit.NewMemStudioBatchRunRepository(), nil, nil
	})
}

func BuildListingKitStudioBatchRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioBatchRepository, func(logger *logrus.Logger) (listingkit.StudioBatchRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit studio batch repository")
		return listingkit.NewMemStudioBatchRepository(), nil, nil
	})
}

func BuildListingAdminStoreRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminStoreRepository, func(logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit store admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingKitStoreProfileRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStoreProfileRepository, func(logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit store profile repository")
		return nil, nil, nil
	})
}

func BuildListingKitStoreRoutingSettingsRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStoreRoutingSettingsRepository, func(logger *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit store routing repository")
		return nil, nil, nil
	})
}

func BuildListingAdminStoreStatisticsRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminStoreStatisticsRepository, func(logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit store statistics admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminImportTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminImportTaskRepository, func(logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit import task admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminFilterRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminFilterRuleRepository, func(logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit filter rule admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminProfitRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminProfitRuleRepository, func(logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit profit rule admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminPricingRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminPricingRuleRepository, func(logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit pricing rule admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminOperationStrategyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminOperationStrategyRepository, func(logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit operation strategy admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminSensitiveWordRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminSensitiveWordRepository, func(logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit sensitive word admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminGenerationTopicPolicyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminGenerationTopicPolicyRepository, func(logger *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit generation topic policy admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminProductImportMappingRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminProductImportMappingRepository, func(logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit product import mapping admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminCategoryRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminCategoryRepository, func(logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit category admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingAdminProductDataRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingAdminProductDataRepository, func(logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
		logger.Warn("database not configured, ListingKit product data admin API disabled")
		return nil, nil, nil
	})
}

func BuildListingSubscriptionRepository(cfg *config.Config, logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingSubscriptionRepository, func(logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory ListingKit subscription repository")
		return listingsubscription.NewMemRepository(), nil, nil
	})
}

func BuildAssetRepository(cfg *config.Config, logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBAssetRepository, func(logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory asset repository")
		return assetrepo.NewMemRepository(), nil, nil
	})
}

func BuildListingKitReviewRepository(cfg *config.Config, logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitReviewRepository, func(logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit review repository")
		return reviewstore.NewMemRepository(), nil, nil
	})
}

func BuildListingKitStudioSessionRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitStudioSessionRepository, func(logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
		logger.Warn("database not configured, SHEIN studio session repository disabled")
		return nil, nil, nil
	})
}

func BuildListingKitUploadedImageRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBListingKitUploadedImageRepository, func(logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
		logger.Warn("database not configured, using in-memory listingkit uploaded image repository")
		return listingkit.NewMemUploadedImageRepository(), nil, nil
	})
}

func BuildSheinResolutionCacheStore(cfg *config.Config, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBSheinResolutionCacheStore, func(logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
		logger.Warn("database not configured, using in-memory SHEIN resolution cache fallback")
		return nil, nil, nil
	})
}

func BuildSheinPricingPolicy(cfg *config.Config) sheinpub.PricingPolicy {
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

func BuildImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	if cfg == nil {
		return nil
	}
	if shouldUseS3ImageUploadStore(cfg) {
		primaryStore := buildS3ImageUploadStore(cfg, logger)
		if primaryStore == nil {
			return nil
		}
		fallbackStore := buildLocalImageUploadStore(cfg, logger)
		store, err := listingkit.NewFallbackImageUploadStore(primaryStore, fallbackStore)
		if err != nil {
			logger.WithError(err).Warn("listingkit image upload store fallback unavailable")
			return primaryStore
		}
		return store
	}
	return buildLocalImageUploadStore(cfg, logger)
}

func shouldUseS3ImageUploadStore(cfg *config.Config) bool {
	return cfg != nil && strings.EqualFold(strings.TrimSpace(cfg.ProductImage.Publisher.Provider), "s3")
}

func localImageUploadRootDir(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	return filepath.Join(cfg.ProductImage.Publisher.OutputDir, "listingkit-inputs")
}

func buildLocalImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	rootDir := localImageUploadRootDir(cfg)
	store, err := listingkit.NewLocalImageUploadStore(rootDir)
	if err != nil {
		if logger != nil {
			logger.WithError(err).Warn("local listingkit image upload store unavailable")
		}
		return nil
	}
	return store
}

func ConfigureLegacyTenantResolver(cfg *config.Config, logger *logrus.Logger) (func() error, error) {
	if shouldDisableLegacyTenantResolver(cfg) {
		tenantbridge.ConfigureLegacyTenantResolver(nil)
		return nil, nil
	}
	for _, zitadelCfg := range legacyTenantResolverDatabaseConfigs(cfg) {
		db, err := database.NewSharedDatabaseFromConfig(&zitadelCfg)
		if err != nil {
			continue
		}
		if !legacyTenantMetadataTableExists(db) {
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

func shouldDisableLegacyTenantResolver(cfg *config.Config) bool {
	return cfg == nil || cfg.Database == nil || strings.TrimSpace(cfg.Database.Host) == ""
}

func legacyTenantResolverDatabaseConfigs(cfg *config.Config) []config.DatabaseConfig {
	if shouldDisableLegacyTenantResolver(cfg) {
		return nil
	}
	candidates := []string{"zitadel_auth", "zitadel"}
	configs := make([]config.DatabaseConfig, 0, len(candidates))
	for _, databaseName := range candidates {
		zitadelCfg := *cfg.Database
		zitadelCfg.Database = databaseName
		configs = append(configs, zitadelCfg)
	}
	return configs
}

func newDBListingKitTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := autoMigrateListingKitTaskRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit auto-migrate failed: %w", err)
	}
	repo := listingkitstore.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func autoMigrateListingKitTaskRepository(db *gorm.DB) error {
	return db.AutoMigrate(
		&listingkit.Task{},
		&listingkit.CanonicalProductCacheEntry{},
		&listingkit.SDSBaselineCacheEntry{},
	)
}

func newDBListingKitStudioAsyncJobRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingkit.AutoMigrateStudioAsyncJobRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit studio async job auto-migrate failed: %w", err)
	}
	repo := listingkit.NewGormStudioAsyncJobRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitStudioBatchRunRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioBatchRunRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingkit.AutoMigrateStudioBatchRunRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit studio batch run auto-migrate failed: %w", err)
	}
	repo := listingkit.NewGormStudioBatchRunRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitStudioBatchRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioBatchRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingkit.AutoMigrateStudioBatchRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit studio batch auto-migrate failed: %w", err)
	}
	repo := listingkit.NewGormStudioBatchRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitUploadedImageRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.UploadedImageRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingkit.AutoMigrateUploadedImageRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit uploaded image auto-migrate failed: %w", err)
	}
	repo := listingkit.NewGormUploadedImageRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitStoreProfileRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StoreProfileRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingkit.AutoMigrateStoreProfileRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit store profile auto-migrate failed: %w", err)
	}
	repo := listingkit.NewGormStoreProfileRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitStoreRoutingSettingsRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingkit.AutoMigrateStoreProfileRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit store routing auto-migrate failed: %w", err)
	}
	repo := listingkit.NewGormStoreRoutingSettingsRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminStoreRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.StoreRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateStoreRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin store auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormStoreRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminStoreStatisticsRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateStoreStatisticsRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin store statistics auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormStoreStatisticsRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminImportTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ImportTaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin import task auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormImportTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminFilterRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.FilterRuleRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateFilterRuleRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin filter rule auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormFilterRuleRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminProfitRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateProfitRuleRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin profit rule auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormProfitRuleRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminPricingRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.PricingRuleRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigratePricingRuleRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin pricing rule auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormPricingRuleRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminOperationStrategyRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateOperationStrategyRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin operation strategy auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormOperationStrategyRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminSensitiveWordRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateSensitiveWordRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin sensitive word auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormSensitiveWordRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminGenerationTopicPolicyRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.GenerationTopicPolicyRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin generation topic policy auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormGenerationTopicPolicyRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminProductImportMappingRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateProductImportMappingRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin product import mapping auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormProductImportMappingRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminCategoryRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.CategoryRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateCategoryRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin category auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormCategoryRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminProductDataRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProductDataRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin product data auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormProductDataRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBSheinResolutionCacheStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&sheinpub.SheinResolutionCacheEntry{}); err != nil {
		return nil, nil, fmt.Errorf("shein resolution cache auto-migrate failed: %w", err)
	}
	store := sheinpub.NewGormResolutionCacheStore(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return store, closer, nil
}

func newDBAssetRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (assetrepo.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&assetrepo.InventorySnapshot{}, &assetrepo.GenerationTaskSnapshot{}); err != nil {
		return nil, nil, fmt.Errorf("asset inventory auto-migrate failed: %w", err)
	}
	repo := assetrepo.NewGormRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitReviewRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (reviewstore.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&reviewstore.ReviewRecord{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit review auto-migrate failed: %w", err)
	}
	repo := reviewstore.NewGormRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitStudioSessionRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioSessionRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit studio session auto-migrate failed: %w", err)
	}
	repo := studiostore.NewGormRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingSubscriptionRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingsubscription.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit subscription auto-migrate failed: %w", err)
	}
	repo := listingsubscription.NewGormRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func buildS3ImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	client, err := newProductImagePublisherS3Client(cfg)
	if err != nil {
		logger.WithError(err).Warn("s3 listingkit image upload store unavailable")
		return nil
	}

	publicBase := strings.TrimSpace(cfg.ProductImage.Publisher.PublicBase)
	if publicBase == "" {
		publicBase = storageinfra.BuildS3PublicBase(
			cfg.ProductImage.Publisher.S3.Endpoint,
			cfg.ProductImage.Publisher.S3.Bucket,
			cfg.ProductImage.Publisher.S3.UsePathStyle,
		)
	}

	store, err := listingkit.NewS3ImageUploadStore(listingkit.S3ImageUploadStoreConfig{
		Bucket:     cfg.ProductImage.Publisher.S3.Bucket,
		PublicBase: publicBase,
		Uploader: storageinfra.NewS3UploaderWithOptions(client, storageinfra.S3UploaderOptions{
			Bucket:       cfg.ProductImage.Publisher.S3.Bucket,
			PublicBase:   publicBase,
			Endpoint:     cfg.ProductImage.Publisher.S3.Endpoint,
			UsePathStyle: cfg.ProductImage.Publisher.S3.UsePathStyle,
		}),
		Reader: client,
	})
	if err != nil {
		logger.WithError(err).Warn("s3 listingkit image upload store unavailable")
		return nil
	}
	return store
}

func newProductImagePublisherS3Client(cfg *config.Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	s3Cfg := cfg.ProductImage.Publisher.S3
	if strings.TrimSpace(s3Cfg.Bucket) == "" {
		return nil, fmt.Errorf("productimage.publisher.s3.bucket cannot be empty")
	}
	return storageinfra.NewS3Client(storageinfra.S3ClientConfig{
		Region:          s3Cfg.Region,
		Endpoint:        s3Cfg.Endpoint,
		AccessKeyID:     s3Cfg.AccessKeyID,
		SecretAccessKey: s3Cfg.SecretAccessKey,
		UsePathStyle:    s3Cfg.UsePathStyle,
	})
}

func legacyTenantMetadataTableExists(db *gorm.DB) bool {
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
