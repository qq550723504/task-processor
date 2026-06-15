package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingkit/studiostore"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

func newDBListingKitTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	repo := listingkit.NewGormStudioBatchRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingKitSheinSyncRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.SheinSyncRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	repo := listingkitstore.NewSheinSyncRepository(db)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	repo := listingkit.NewGormStoreProfileRepository(db)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	repo := listingadmin.NewGormGenerationTopicPolicyRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBListingAdminGenerationTopicOverrideRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.GenerationTopicOverrideRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	repo := listingadmin.NewGormGenerationTopicOverrideRepository(db)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
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
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	repo := listingsubscription.NewGormRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}
