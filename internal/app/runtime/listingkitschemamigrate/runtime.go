package listingkitschemamigrate

import (
	"context"
	"flag"
	"fmt"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	listingkitschema "task-processor/internal/listingkit/schema"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/pkg/appenv"
	platformdatabase "task-processor/internal/platform/database"
	workbenchschema "task-processor/internal/workbench/schema"

	"gorm.io/gorm"
)

type runtimeDependencies struct {
	LoadConfig       func(configPath string) (*config.Config, error)
	OpenDB           func(cfg *config.DatabaseConfig) (*gorm.DB, error)
	CloseDB          func(db *gorm.DB) error
	MigrateAll       func(db *gorm.DB) error
	MigrateSheinSync func(db *gorm.DB) error
	MigrateWorkbench func(db *gorm.DB) error
}

func defaultRuntimeDependencies() runtimeDependencies {
	return runtimeDependencies{
		LoadConfig: config.LoadConfigFromFileWithoutValidation,
		OpenDB: func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.Open(configadapter.Database(cfg))
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
		MigrateWorkbench: workbenchschema.AutoMigrateRuntime,
	}
}

func Run(ctx context.Context, opts Options) error {
	return runWithDependencies(ctx, opts, defaultRuntimeDependencies())
}

func runWithDependencies(ctx context.Context, opts Options, deps runtimeDependencies) error {
	if err := validateMigrationScope(opts.Scope); err != nil {
		return err
	}
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
	if deps.MigrateWorkbench == nil {
		deps.MigrateWorkbench = defaults.MigrateWorkbench
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
	if err := validateMigrationScope(scope); err != nil {
		return err
	}
	switch scope {
	case "", "all":
		if err := deps.MigrateAll(db); err != nil {
			return err
		}
		return deps.MigrateWorkbench(db)
	case "shein-sync":
		return deps.MigrateSheinSync(db)
	case "workbench":
		return deps.MigrateWorkbench(db)
	default:
		return flag.ErrHelp
	}
}

func validateMigrationScope(scope string) error {
	switch scope {
	case "", "all", "shein-sync", "workbench":
		return nil
	default:
		return flag.ErrHelp
	}
}

func autoMigrateListingKitRuntimeSchema(db *gorm.DB) error {
	return listingkitschema.AutoMigrateRuntime(db)
}
