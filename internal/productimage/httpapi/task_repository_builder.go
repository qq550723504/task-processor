package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
)

func buildTaskRepository(cfg *config.Config, logger *logrus.Logger) (productimage.TaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create image task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory productimage repository")
	return productimagestore.NewMemTaskRepository(), nil, nil
}

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productimage.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		return nil, nil, fmt.Errorf("productimage auto-migrate failed: %w", err)
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
}
