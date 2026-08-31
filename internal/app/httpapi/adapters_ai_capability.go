package httpapi

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

func newDBAICapabilityStores(cfg *config.DatabaseConfig, logger *logrus.Logger, featureFlags BoolEvaluator) (aicapability.InvocationRecorder, aicapability.AsyncJobBindingStore, func() error, error) {
	if cfg == nil {
		return nil, nil, nil, fmt.Errorf("AI capability invocation ledger database config is nil")
	}
	databaseConfig := configadapter.Database(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("AI capability invocation ledger database connection failed (%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	if logger != nil {
		logger.Infof("AI capability invocation ledger database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	}
	if shouldAutoMigrateProductListingAPIRuntime(context.Background(), featureFlags) {
		if err := aicapabilitystore.AutoMigrateInvocationLedger(db); err != nil {
			_ = platformdatabase.CloseShared(databaseConfig, db)
			return nil, nil, nil, fmt.Errorf("ai invocation ledger auto-migrate failed: %w", err)
		}
		if err := aicapabilitystore.AutoMigrateAsyncJobBindings(db); err != nil {
			_ = platformdatabase.CloseShared(databaseConfig, db)
			return nil, nil, nil, fmt.Errorf("ai async job binding auto-migrate failed: %w", err)
		}
	}
	return aicapabilitystore.NewGormInvocationRecorder(db), aicapabilitystore.NewGormAsyncJobBindingStore(db), func() error {
		return platformdatabase.CloseShared(databaseConfig, db)
	}, nil
}
