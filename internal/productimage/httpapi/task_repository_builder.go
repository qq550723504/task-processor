package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
)

func buildTaskRepository(databaseConfig *config.DatabaseConfig, logger *logrus.Logger) (productimage.TaskRepository, []func() error, error) {
	if databaseConfig != nil && databaseConfig.Host != "" {
		repo, closer, err := newDBTaskRepository(databaseConfig, logger)
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
	databaseConfig := platformDatabaseConfig(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if config.ProductListingAPIRuntimeAutoMigrateEnabled() {
		if err := db.AutoMigrate(&productimage.Task{}); err != nil {
			return nil, nil, fmt.Errorf("productimage auto-migrate failed: %w", err)
		}
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return platformdatabase.CloseShared(databaseConfig, db) }
	return repo, closer, nil
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
