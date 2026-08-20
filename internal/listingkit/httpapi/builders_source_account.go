package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/sourceaccount"
)

func BuildSourceAccountRepository(cfg *config.Config, logger *logrus.Logger) (sourceaccount.Repository, []func() error, error) {
	return buildRepositoryWithFallback(cfg, logger, newDBSourceAccountRepository, func(_ *logrus.Logger) (sourceaccount.Repository, []func() error, error) {
		return nil, nil, nil
	})
}
