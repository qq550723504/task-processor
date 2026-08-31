package httpapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/app/schema/productlisting"
	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

type productListingSchemaMigrator func(context.Context, *config.DatabaseConfig, *logrus.Logger) error

func AutoMigrateProductListingAPIRuntimeSchema(db *gorm.DB) error {
	return productlisting.Migrate(context.Background(), db)
}

func migrateProductListingAPIRuntimeSchema(ctx context.Context, cfg *config.DatabaseConfig, logger *logrus.Logger) error {
	if cfg == nil {
		return fmt.Errorf("database config is nil")
	}
	databaseConfig := configadapter.Database(cfg)
	db, err := platformdatabase.Open(databaseConfig)
	if err != nil {
		return fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	if logger != nil {
		logger.Infof("database connected for product listing schema migration: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	}
	migrateErr := productlisting.Migrate(ctx, db)
	closeErr := platformdatabase.Close(db)
	if migrateErr != nil {
		return errors.Join(migrateErr, closeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close product listing schema database: %w", closeErr)
	}
	return nil
}

func migrateProductListingSchemaIfEnabled(ctx context.Context, evaluator BoolEvaluator, cfg *config.DatabaseConfig, logger *logrus.Logger, migrateSchema productListingSchemaMigrator) error {
	if cfg == nil || !shouldAutoMigrateProductListingAPIRuntime(ctx, evaluator) {
		return nil
	}
	if migrateSchema == nil {
		return fmt.Errorf("product listing schema migrator is nil")
	}
	if err := migrateSchema(ctx, cfg, logger); err != nil {
		return fmt.Errorf("migrate product listing API schema: %w", err)
	}
	return nil
}
