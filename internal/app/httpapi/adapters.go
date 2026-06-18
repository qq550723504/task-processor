package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	bootstrapresources "task-processor/internal/app/bootstrap/resources"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/prompt"
)

func newDBTenantPromptStore(cfg *config.DatabaseConfig, logger *logrus.Logger) (prompt.TenantPromptStore, func() error, error) {
	if !shouldAutoMigrateProductListingAPIRuntime() {
		if cfg == nil {
			return nil, nil, fmt.Errorf("database config is nil")
		}
		db, err := database.NewSharedDatabaseFromConfig(cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
		}
		logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)
		store := prompt.NewGormTenantPromptStore(db)
		closer := func() error { return database.CloseSharedDatabase(cfg, db) }
		return store, closer, nil
	}
	return bootstrapresources.NewDBTenantPromptStore(cfg, logger)
}

func newWebScraper(cfg *config.Config) productenrich.WebScraper {
	return productenrichenrich.NewCrawler1688Adapter(cfg)
}

type poolSubmitter struct {
	pool worker.WorkerPool
}

func (s *poolSubmitter) Submit(taskID string) error {
	return s.pool.Submit(worker.WorkerJob{TaskData: taskID})
}

func newRedisClient(cfg *config.RedisConfig, logger *logrus.Logger) (productenrich.RedisClient, error) {
	rc, err := redisclient.New(cfg)
	if err != nil {
		return nil, err
	}
	logger.Infof("Redis connected: %s:%d db=%d", cfg.Host, cfg.Port, cfg.DB)
	return rc, nil
}
