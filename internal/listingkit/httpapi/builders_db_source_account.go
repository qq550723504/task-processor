package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/sourceaccount"
)

func newDBSourceAccountRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (sourceaccount.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	if err := sourceaccount.AutoMigrateRepository(db); err != nil {
		_ = closer()
		return nil, nil, fmt.Errorf("source account schema bootstrap failed: %w", err)
	}
	return sourceaccount.NewGormRepository(db), closer, nil
}
