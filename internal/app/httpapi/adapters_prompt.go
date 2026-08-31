package httpapi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
	"task-processor/internal/prompt"
)

func newDBTenantPromptStore(cfg *config.DatabaseConfig, logger *logrus.Logger, featureFlags BoolEvaluator) (prompt.TenantPromptStore, func() error, error) {
	if !shouldAutoMigrateProductListingAPIRuntime(context.Background(), featureFlags) {
		if cfg == nil {
			return nil, nil, fmt.Errorf("database config is nil")
		}
		databaseConfig := configadapter.Database(cfg)
		db, err := platformdatabase.OpenShared(databaseConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
		}
		logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
		store := prompt.NewGormTenantPromptStore(db)
		closer := func() error { return platformdatabase.CloseShared(databaseConfig, db) }
		return store, closer, nil
	}
	return bootstrapresources.NewDBTenantPromptStore(cfg, logger)
}
