package listingkitschemamigrate

import (
	"context"
	"flag"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	listingkitschema "task-processor/internal/listingkit/schema"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/pkg/appenv"

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
		LoadConfig: config.LoadConfigFromFileWithoutValidation,
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
	return listingkitschema.AutoMigrateRuntime(db)
}
