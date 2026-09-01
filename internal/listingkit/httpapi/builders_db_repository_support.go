package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

func openListingKitRepositoryDB(cfg *config.DatabaseConfig, logger *logrus.Logger) (*gorm.DB, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	databaseConfig := platformDatabaseConfig(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
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
