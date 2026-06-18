package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/productenrich"
	"task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
)

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productenrich.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if shouldAutoMigrateProductListingAPIRuntime() {
		if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
			return nil, nil, fmt.Errorf("database auto-migrate failed: %w", err)
		}
	}

	repo := store.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBImageTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productimage.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if shouldAutoMigrateProductListingAPIRuntime() {
		if err := db.AutoMigrate(&productimage.Task{}); err != nil {
			return nil, nil, fmt.Errorf("productimage auto-migrate failed: %w", err)
		}
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}

func newDBAmazonListingTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (amazonlisting.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if shouldAutoMigrateProductListingAPIRuntime() {
		if err := db.AutoMigrate(&amazonlisting.Task{}); err != nil {
			return nil, nil, fmt.Errorf("amazonlisting auto-migrate failed: %w", err)
		}
	}

	repo := amazonlistingstore.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}
