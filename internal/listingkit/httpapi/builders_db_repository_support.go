package httpapi

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingkit/studiostore"
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
	sheinpub "task-processor/internal/publishing/shein"
)

type repositoryDatabaseOpener func(*config.DatabaseConfig, *logrus.Logger) (*gorm.DB, func() error, error)

// BuildPersistentRepositories opens one database ownership and constructs the
// complete repository value set consumed by ListingKit.
func BuildPersistentRepositories(cfg *config.DatabaseConfig, logger *logrus.Logger) (BuildServiceRepositories, func() error, error) {
	return buildPersistentRepositoriesWithOpener(cfg, logger, openListingKitRepositoryDB)
}

func buildPersistentRepositoriesWithOpener(cfg *config.DatabaseConfig, logger *logrus.Logger, open repositoryDatabaseOpener) (BuildServiceRepositories, func() error, error) {
	if cfg == nil || strings.TrimSpace(cfg.Host) == "" {
		return BuildServiceRepositories{}, nil, fmt.Errorf("listingkit database config is required")
	}
	if open == nil {
		return BuildServiceRepositories{}, nil, fmt.Errorf("listingkit repository database opener is required")
	}
	db, closer, err := open(cfg, logger)
	if err != nil {
		return BuildServiceRepositories{}, nil, err
	}
	repositories, err := NewPersistentRepositories(db)
	if err != nil {
		if closer != nil {
			_ = closer()
		}
		return BuildServiceRepositories{}, nil, err
	}
	if closer == nil {
		return BuildServiceRepositories{}, nil, fmt.Errorf("listingkit repository database closer is required")
	}
	return repositories, closer, nil
}

// NewPersistentRepositories constructs the complete typed repository set from
// a caller-owned database. Tests use this with an explicit SQLite fixture.
func NewPersistentRepositories(db *gorm.DB) (BuildServiceRepositories, error) {
	if db == nil {
		return BuildServiceRepositories{}, fmt.Errorf("listingkit repository database is required")
	}
	approvedAssets, err := assetpersistence.NewRepository(db)
	if err != nil {
		return BuildServiceRepositories{}, fmt.Errorf("create approved asset repository: %w", err)
	}
	subscriptions := listingsubscription.NewGormRepository(db)
	return BuildServiceRepositories{
		Core: CoreRepositories{
			Task:                  listingkitstore.NewTaskRepository(db),
			StudioAsyncJob:        listingkit.NewGormStudioAsyncJobRepository(db),
			StudioBatch:           listingkit.NewGormStudioBatchRepository(db),
			StudioBatchRun:        listingkit.NewGormStudioBatchRunRepository(db),
			SheinSync:             listingkitstore.NewSheinSyncRepository(db),
			Subscription:          subscriptions,
			GenerationUsageLedger: listingsubscription.NewGormUsageLedger(subscriptions),
			MemberInvitationAudit: memberinvite.NewGormAuditRepository(db),
			ApprovedAsset:         approvedAssets,
			Review:                reviewstore.NewGormRepository(db),
			StudioSession:         studiostore.NewGormRepository(db),
			UploadedImage:         listingkit.NewGormUploadedImageRepository(db),
			StoreProfile:          listingkit.NewGormStoreProfileRepository(db),
			SheinResolutionCache:  sheinpub.NewGormResolutionCacheStore(db),
		},
		Admin: AdminRepositories{
			Store:                   listingadmin.NewGormStoreRepository(db),
			StoreStatistics:         listingadmin.NewGormStoreStatisticsRepository(db),
			DispatchEvent:           listingadmin.NewGormDispatchEventRepository(db),
			ImportTask:              listingadmin.NewGormImportTaskRepository(db),
			FilterRule:              listingadmin.NewGormFilterRuleRepository(db),
			ProfitRule:              listingadmin.NewGormProfitRuleRepository(db),
			PricingRule:             listingadmin.NewGormPricingRuleRepository(db),
			OperationStrategy:       listingadmin.NewGormOperationStrategyRepository(db),
			ScheduledTaskConfig:     listingadmin.NewGormScheduledTaskConfigRepository(db),
			SensitiveWord:           listingadmin.NewGormSensitiveWordRepository(db),
			GenerationTopicOverride: listingadmin.NewGormGenerationTopicOverrideRepository(db),
			GenerationTopicPolicy:   listingadmin.NewGormGenerationTopicPolicyRepository(db),
			ProductImportMapping:    listingadmin.NewGormProductImportMappingRepository(db),
			Category:                listingadmin.NewGormCategoryRepository(db),
			ProductData:             listingadmin.NewGormProductDataRepository(db),
		},
	}, nil
}

func openListingKitRepositoryDB(cfg *config.DatabaseConfig, logger *logrus.Logger) (*gorm.DB, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	databaseConfig := platformDatabaseConfig(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	if logger != nil {
		logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	}
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		_ = platformdatabase.CloseShared(databaseConfig, db)
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	return db, func() error { return platformdatabase.CloseShared(databaseConfig, db) }, nil
}

func platformDatabaseConfig(cfg *config.DatabaseConfig) *platformdatabase.Config {
	if cfg == nil {
		return nil
	}
	return &platformdatabase.Config{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		Database:              cfg.Database,
		MaxConnections:        cfg.MaxConnections,
		MaxIdleConnections:    cfg.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.ConnectionMaxLifetime,
	}
}
