package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	platformdatabase "task-processor/internal/platform/database"
)

func newDBAICapabilityStores(cfg *config.DatabaseConfig, logger *logrus.Logger) (aicapability.InvocationRecorder, aicapability.AsyncJobBindingStore, func() error, error) {
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
	return aicapabilitystore.NewGormInvocationRecorder(db), aicapabilitystore.NewGormAsyncJobBindingStore(db), func() error {
		return platformdatabase.CloseShared(databaseConfig, db)
	}, nil
}
