package listingkitschemamigrate

import (
	"context"
	"flag"
	"fmt"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/pkg/appenv"
	sheinpub "task-processor/internal/publishing/shein"

	"gorm.io/gorm"
)

type runtimeDependencies struct {
	LoadConfig       func(configPath string) (*config.Config, error)
	OpenDB           func(cfg *config.DatabaseConfig) (*gorm.DB, error)
	CloseDB          func(db *gorm.DB) error
	MigrateAll       func(db *gorm.DB) error
	MigrateSheinSync func(db *gorm.DB) error
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigFromFile,
		OpenDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return database.NewDatabaseFromConfig(cfg)
		},
		CloseDB: func(db *gorm.DB) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.Close()
		},
		MigrateAll: func(db *gorm.DB) error {
			return autoMigrateListingKitRuntimeSchema(db)
		},
		MigrateSheinSync: func(db *gorm.DB) error {
			return listingkitstore.AutoMigrateSheinSyncRepository(db)
		},
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
	_ = ctx
	defaults := defaultRuntimeDependencies()
	if deps.LoadConfig == nil {
		deps.LoadConfig = defaults.LoadConfig
	}
	if deps.OpenDB == nil {
		deps.OpenDB = defaults.OpenDB
	}
	if deps.CloseDB == nil {
		deps.CloseDB = defaults.CloseDB
	}
	if deps.MigrateAll == nil {
		deps.MigrateAll = defaults.MigrateAll
	}
	if deps.MigrateSheinSync == nil {
		deps.MigrateSheinSync = defaults.MigrateSheinSync
	}

	logger := appenv.SetupLoggerWithLevel(opts.LogLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{Version: opts.Version, BuildTime: opts.BuildTime})

	cfg, err := deps.LoadConfig(opts.ConfigPath())
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	db, err := deps.OpenDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database failed: %w", err)
	}
	if db == nil {
		return fmt.Errorf("database config is required")
	}
	defer func() {
		if err := deps.CloseDB(db); err != nil {
			logger.WithError(err).Warn("close database failed")
		}
	}()

	if err := runMigration(db, opts.Scope, deps); err != nil {
		return fmt.Errorf("listingkit schema migrate failed: %w", err)
	}
	logger.WithField("scope", opts.Scope).Info("listingkit schema migrate completed")
	return nil
}

func runMigration(db *gorm.DB, scope string, deps runtimeDependencies) error {
	switch scope {
	case "", "all":
		return deps.MigrateAll(db)
	case "shein-sync":
		return deps.MigrateSheinSync(db)
	default:
		return flag.ErrHelp
	}
}

func autoMigrateListingKitRuntimeSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := autoMigrateListingKitTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit task repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioAsyncJobRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio async job repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchRunRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch run repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchTaskLinkRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch task link repository: %w", err)
	}
	if err := listingkitstore.AutoMigrateSheinSyncRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit shein sync repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSRetirementRunRecord{}, &listingkit.SDSRetirementItemRecord{}); err != nil {
		return fmt.Errorf("migrate listingkit sds retirement repository: %w", err)
	}
	if err := listingkit.AutoMigrateUploadedImageRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit uploaded image repository: %w", err)
	}
	if err := listingkit.AutoMigrateStoreProfileRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit store profile repository: %w", err)
	}
	if err := listingadmin.AutoMigrateStoreRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin store repository: %w", err)
	}
	if err := listingadmin.AutoMigrateStoreStatisticsRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin store statistics repository: %w", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin import task repository: %w", err)
	}
	if err := listingadmin.AutoMigrateFilterRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin filter rule repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProfitRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin profit rule repository: %w", err)
	}
	if err := listingadmin.AutoMigratePricingRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin pricing rule repository: %w", err)
	}
	if err := listingadmin.AutoMigrateOperationStrategyRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin operation strategy repository: %w", err)
	}
	if err := listingadmin.AutoMigrateScheduledTaskConfigRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin scheduled task config repository: %w", err)
	}
	if err := listingadmin.AutoMigrateSensitiveWordRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin sensitive word repository: %w", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin generation topic policy repository: %w", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicOverrideRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin generation topic override repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProductImportMappingRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin product import mapping repository: %w", err)
	}
	if err := listingadmin.AutoMigrateCategoryRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin category repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin product data repository: %w", err)
	}
	if err := db.AutoMigrate(&sheinpub.SheinResolutionCacheEntry{}); err != nil {
		return fmt.Errorf("migrate shein resolution cache store: %w", err)
	}
	if err := db.AutoMigrate(&assetrepo.InventorySnapshot{}, &assetrepo.GenerationTaskSnapshot{}); err != nil {
		return fmt.Errorf("migrate asset repository: %w", err)
	}
	if err := db.AutoMigrate(&reviewstore.ReviewRecord{}); err != nil {
		return fmt.Errorf("migrate listingkit review repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		return fmt.Errorf("migrate listingkit studio session repository: %w", err)
	}
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit subscription repository: %w", err)
	}
	return nil
}

func autoMigrateListingKitTaskRepository(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&listingkit.Task{},
		&listingkit.CanonicalProductCacheEntry{},
		&listingkit.SDSBaselineCacheEntry{},
	); err != nil {
		return err
	}
	return listingkitstore.AutoMigrateSheinPODImageLookupIndex(db)
}
