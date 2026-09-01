package httpapi

import (
	"github.com/sirupsen/logrus"

	"task-processor/internal/app/configadapter"
	"task-processor/internal/core/config"
	platformredis "task-processor/internal/platform/redis"
	worker "task-processor/internal/platform/workerpool"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
)

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
	rc, err := platformredis.New(configadapter.Redis(cfg))
	if err != nil {
		return nil, err
	}
	logger.Infof("Redis connected: %s:%d db=%d", cfg.Host, cfg.Port, cfg.DB)
	return rc, nil
}
