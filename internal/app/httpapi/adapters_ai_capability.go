package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
)

func newDBAICapabilityStores(cfg *config.DatabaseConfig, logger *logrus.Logger) (aicapability.InvocationRecorder, aicapability.AsyncJobBindingStore, func() error, error) {
	if cfg == nil {
		return nil, nil, nil, fmt.Errorf("AI capability invocation ledger database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("AI capability invocation ledger database connection failed (%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	if logger != nil {
		logger.Infof("AI capability invocation ledger database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
	}
	if shouldAutoMigrateProductListingAPIRuntime() {
		if err := aicapabilitystore.AutoMigrateInvocationLedger(db); err != nil {
			_ = database.CloseSharedDatabase(cfg, db)
			return nil, nil, nil, fmt.Errorf("ai invocation ledger auto-migrate failed: %w", err)
		}
		if err := aicapabilitystore.AutoMigrateAsyncJobBindings(db); err != nil {
			_ = database.CloseSharedDatabase(cfg, db)
			return nil, nil, nil, fmt.Errorf("ai async job binding auto-migrate failed: %w", err)
		}
	}
	return aicapabilitystore.NewGormInvocationRecorder(db), aicapabilitystore.NewGormAsyncJobBindingStore(db), func() error {
		return database.CloseSharedDatabase(cfg, db)
	}, nil
}
