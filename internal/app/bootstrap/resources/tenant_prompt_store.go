package resources

import (
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/prompt"

	"github.com/sirupsen/logrus"
)

func NewDBTenantPromptStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (prompt.TenantPromptStore, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}

	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}

	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	if err := db.AutoMigrate(&prompt.TenantPromptTemplate{}); err != nil {
		return nil, nil, fmt.Errorf("tenant prompt auto-migrate failed: %w", err)
	}

	store := prompt.NewGormTenantPromptStore(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return store, closer, nil
}
