package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"task-processor/internal/core/config"
	"task-processor/internal/sourceaccount"
)

// newDBSourceAccountRepository remains as a compatibility seam for the
// existing HTTPAPI builder boundary. New marketplace runtimes should use
// internal/sourceaccount/bootstrap directly.
func newDBSourceAccountRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (sourceaccount.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	if err := autoMigrateSourceAccountRepository(db); err != nil {
		_ = closer()
		return nil, nil, fmt.Errorf("source account schema bootstrap failed: %w", err)
	}
	return sourceaccount.NewGormRepository(db), closer, nil
}

func autoMigrateSourceAccountRepository(db *gorm.DB) error {
	if !shouldAutoMigrateListingKitRuntime() {
		return nil
	}
	return sourceaccount.AutoMigrateRepository(db)
}
