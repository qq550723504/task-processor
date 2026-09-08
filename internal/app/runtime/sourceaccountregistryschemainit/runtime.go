package sourceaccountregistryschemainit

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"task-processor/internal/app/configadapter"
	registryschema "task-processor/internal/app/schema/sourceaccountregistry"
	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

type Dependencies struct {
	LoadConfig func(string) (*config.Config, error)
	OpenDB     func(*config.DatabaseConfig) (*gorm.DB, error)
	Migrate    func(context.Context, *gorm.DB) error
	CloseDB    func(*gorm.DB) error
}

func Run(ctx context.Context, configPath string) error {
	return runWithDependencies(ctx, configPath, Dependencies{})
}

func runWithDependencies(ctx context.Context, configPath string, dependencies Dependencies) error {
	if dependencies.LoadConfig == nil {
		dependencies.LoadConfig = config.LoadConfigFromFileWithoutValidation
	}
	if dependencies.OpenDB == nil {
		dependencies.OpenDB = func(database *config.DatabaseConfig) (*gorm.DB, error) {
			return platformdatabase.Open(configadapter.Database(database))
		}
	}
	if dependencies.Migrate == nil {
		dependencies.Migrate = registryschema.Migrate
	}
	if dependencies.CloseDB == nil {
		dependencies.CloseDB = platformdatabase.Close
	}
	cfg, err := dependencies.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg == nil || cfg.Database == nil {
		return fmt.Errorf("database is not configured")
	}
	db, err := dependencies.OpenDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	migrateErr := dependencies.Migrate(ctx, db)
	closeErr := dependencies.CloseDB(db)
	if migrateErr != nil {
		return fmt.Errorf("initialize source account registry schema: %w", errors.Join(migrateErr, closeErr))
	}
	if closeErr != nil {
		return fmt.Errorf("close source account registry database: %w", closeErr)
	}
	return nil
}
