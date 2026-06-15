package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
)

func buildRepositoryWithFallback[T any](
	cfg *config.Config,
	logger *logrus.Logger,
	buildDB func(*config.DatabaseConfig, *logrus.Logger) (T, func() error, error),
	buildFallback func(*logrus.Logger) (T, []func() error, error),
) (T, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := buildDB(cfg.Database, logger)
		if err != nil {
			var zero T
			return zero, nil, err
		}
		return repo, []func() error{closer}, nil
	}
	return buildFallback(logger)
}
