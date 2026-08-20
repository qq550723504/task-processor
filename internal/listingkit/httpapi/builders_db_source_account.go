package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/sourceaccount"
)

func newDBSourceAccountRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (sourceaccount.Repository, func() error, error) {
	db, closer, err := openListingKitRepositoryDB(cfg, logger)
	if err != nil {
		return nil, nil, err
	}
	return sourceaccount.NewGormRepository(db), closer, nil
}
