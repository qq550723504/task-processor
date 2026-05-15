package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingkit/studiostore"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
	"task-processor/internal/prompt"
	sheinpub "task-processor/internal/publishing/shein"
)

func newLLMManager(cfg config.OpenAIConfig) (productenrich.LLMManager, error) {
	manager, err := newOpenAIManager(cfg)
	if err != nil {
		return nil, err
	}
	return productenrich.NewLLMManagerAdapterFromManager(manager)
}

func newOpenAIManager(cfg config.OpenAIConfig) (*openaiclient.Manager, error) {
	return openaiclient.NewManager(&openaiclient.ManagerConfig{
		Clients:       cfg.ToClientConfigs(),
		DefaultClient: "default",
	})
}

func newDBOpenAICredentialResolver(cfg *config.DatabaseConfig, logger *logrus.Logger) (*openaiclient.GormCredentialResolver, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&openaiclient.AIClientCredential{}); err != nil {
		return nil, nil, fmt.Errorf("openai credential auto-migrate failed: %w", err)
	}
	resolver := openaiclient.NewGormCredentialResolver(db)
	closer := func() error { return database.CloseDatabase(db) }
	return resolver, closer, nil
}

func newDBTenantPromptStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (prompt.TenantPromptStore, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&prompt.TenantPromptTemplate{}); err != nil {
		return nil, nil, fmt.Errorf("tenant prompt auto-migrate failed: %w", err)
	}
	store := prompt.NewGormTenantPromptStore(db)
	closer := func() error { return database.CloseDatabase(db) }
	return store, closer, nil
}

func newWebScraper(cfg *config.Config) productenrich.WebScraper {
	return productenrichenrich.NewCrawler1688Adapter(cfg)
}

type poolSubmitter struct {
	pool worker.WorkerPool
}

func (s *poolSubmitter) Submit(taskID string) error {
	return s.pool.Submit(worker.WorkerJob{TaskData: taskID})
}

func newRedisClient(cfg *config.RedisConfig, logger *logrus.Logger) (productenrich.RedisClient, error) {
	rc, err := redisclient.New(cfg)
	if err != nil {
		return nil, err
	}
	logger.Infof("Redis connected: %s:%d db=%d", cfg.Host, cfg.Port, cfg.DB)
	return rc, nil
}

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productenrich.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
		return nil, nil, fmt.Errorf("database auto-migrate failed: %w", err)
	}

	repo := store.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBImageTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productimage.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		return nil, nil, fmt.Errorf("productimage auto-migrate failed: %w", err)
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBAmazonListingTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (amazonlisting.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&amazonlisting.Task{}); err != nil {
		return nil, nil, fmt.Errorf("amazonlisting auto-migrate failed: %w", err)
	}

	repo := amazonlistingstore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingKitTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&listingkit.Task{}, &listingkit.CanonicalProductCacheEntry{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit auto-migrate failed: %w", err)
	}

	repo := listingkitstore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingKitUploadedImageRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.UploadedImageRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := listingkit.AutoMigrateUploadedImageRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit uploaded image auto-migrate failed: %w", err)
	}

	repo := listingkit.NewGormUploadedImageRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminStoreRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.StoreRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := listingadmin.AutoMigrateStoreRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin store auto-migrate failed: %w", err)
	}

	repo := listingadmin.NewGormStoreRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminStoreStatisticsRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := listingadmin.AutoMigrateStoreStatisticsRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin store statistics auto-migrate failed: %w", err)
	}

	repo := listingadmin.NewGormStoreStatisticsRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminImportTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ImportTaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin import task auto-migrate failed: %w", err)
	}

	repo := listingadmin.NewGormImportTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminFilterRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.FilterRuleRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := listingadmin.AutoMigrateFilterRuleRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin filter rule auto-migrate failed: %w", err)
	}

	repo := listingadmin.NewGormFilterRuleRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminProfitRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateProfitRuleRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin profit rule auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormProfitRuleRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminPricingRuleRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.PricingRuleRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigratePricingRuleRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin pricing rule auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormPricingRuleRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminOperationStrategyRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateOperationStrategyRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin operation strategy auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormOperationStrategyRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminSensitiveWordRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateSensitiveWordRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin sensitive word auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormSensitiveWordRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminProductImportMappingRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateProductImportMappingRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin product import mapping auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormProductImportMappingRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminCategoryRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.CategoryRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateCategoryRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin category auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormCategoryRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingAdminProductDataRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingadmin.ProductDataRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listing admin product data auto-migrate failed: %w", err)
	}
	repo := listingadmin.NewGormProductDataRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBSheinResolutionCacheStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&sheinpub.SheinResolutionCacheEntry{}); err != nil {
		return nil, nil, fmt.Errorf("shein resolution cache auto-migrate failed: %w", err)
	}

	store := sheinpub.NewGormResolutionCacheStore(db)
	closer := func() error { return database.CloseDatabase(db) }
	return store, closer, nil
}

func newDBAssetRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (assetrepo.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&assetrepo.InventorySnapshot{}, &assetrepo.GenerationTaskSnapshot{}); err != nil {
		return nil, nil, fmt.Errorf("asset inventory auto-migrate failed: %w", err)
	}

	repo := assetrepo.NewGormRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingKitReviewRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (reviewstore.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&reviewstore.ReviewRecord{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit review auto-migrate failed: %w", err)
	}

	repo := reviewstore.NewGormRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingKitStudioSessionRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.StudioSessionRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit studio session auto-migrate failed: %w", err)
	}

	repo := studiostore.NewGormRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingSubscriptionRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingsubscription.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		return nil, nil, fmt.Errorf("listingkit subscription auto-migrate failed: %w", err)
	}
	repo := listingsubscription.NewGormRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}
