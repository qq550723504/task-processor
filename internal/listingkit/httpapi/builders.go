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

func BuildListingKitTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
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

func BuildListingKitStudioAsyncJobRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioAsyncJobRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStudioAsyncJobRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit studio async job repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit studio async job repository")
	return listingkit.NewMemStudioAsyncJobRepository(), nil, nil
}

func BuildListingAdminStoreRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
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

func BuildListingKitStoreProfileRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
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

func BuildListingKitStoreRoutingSettingsRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
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

func BuildListingAdminStoreStatisticsRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
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

func BuildListingAdminImportTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
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

func BuildListingAdminFilterRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
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

func BuildListingAdminProfitRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
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

func BuildListingAdminPricingRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
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

func BuildListingAdminOperationStrategyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
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

func BuildListingAdminSensitiveWordRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
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

func BuildListingAdminProductImportMappingRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
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

func BuildListingAdminCategoryRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
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

func BuildListingAdminProductDataRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
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

func BuildListingSubscriptionRepository(cfg *config.Config, logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
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

func BuildAssetRepository(cfg *config.Config, logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
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

func BuildListingKitReviewRepository(cfg *config.Config, logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
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

func BuildListingKitStudioSessionRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
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

func BuildListingKitUploadedImageRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
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

func BuildSheinResolutionCacheStore(cfg *config.Config, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
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
	if strings.EqualFold(strings.TrimSpace(cfg.ProductImage.Publisher.Provider), "s3") {
		return buildS3ImageUploadStore(cfg, logger)
	}
	rootDir := filepath.Join(cfg.ProductImage.Publisher.OutputDir, "listingkit-inputs")
	store, err := listingkit.NewLocalImageUploadStore(rootDir)
	if err != nil {
		return nil
	}
	return store
}

func ConfigureLegacyTenantResolver(cfg *config.Config, logger *logrus.Logger) (func() error, error) {
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

func newDBListingKitTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.CanonicalProductCacheEntry{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit auto-migrate failed: %w", err)
	}
	repo := listingkitstore.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
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
