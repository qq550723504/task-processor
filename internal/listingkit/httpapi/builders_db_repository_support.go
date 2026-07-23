package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
)

func openListingKitRepositoryDB(cfg *config.DatabaseConfig, logger *logrus.Logger) (*gorm.DB, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := ensureListingKitRepositorySchema(cfg, db); err != nil {
		_ = database.CloseSharedDatabase(cfg, db)
		return nil, nil, fmt.Errorf("listingkit schema bootstrap failed: %w", err)
	}
	return db, func() error { return database.CloseSharedDatabase(cfg, db) }, nil
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
