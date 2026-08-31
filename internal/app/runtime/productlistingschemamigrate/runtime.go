package productlistingschemamigrate

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/app/schema/productlisting"
	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

type Dependencies struct {
	LoadConfig func(string) (*config.Config, error)
	OpenDB     func(*config.DatabaseConfig) (*gorm.DB, error)
	CloseDB    func(*gorm.DB) error
	MigrateAll func(context.Context, *gorm.DB) error
}

func Run(ctx context.Context, configPath string) error {
	return runWithDependencies(ctx, configPath, Dependencies{})
}

func runWithDependencies(ctx context.Context, configPath string, deps Dependencies) error {
	if deps.LoadConfig == nil {
		deps.LoadConfig = config.LoadConfigFromFileWithoutValidation
	}
	if deps.OpenDB == nil {
		deps.OpenDB = func(cfg *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.Open(configadapter.Database(cfg))
		}
	}
	if deps.CloseDB == nil {
		deps.CloseDB = closeDB
	}
	if deps.MigrateAll == nil {
		deps.MigrateAll = productlisting.Migrate
	}
	cfg, err := deps.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return fmt.Errorf("database is not configured")
	}
	db, err := deps.OpenDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	defer func() { _ = deps.CloseDB(db) }()
	if err := deps.MigrateAll(ctx, db); err != nil {
		return fmt.Errorf("migrate product listing API schema: %w", err)
	}
	return nil
}

func closeDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
