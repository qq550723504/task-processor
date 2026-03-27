package main

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
)

// newLLMManager 创建 OpenAI LLMManager（委托给 productenrich 包）。
func newLLMManager(cfg config.OpenAIConfig) (productenrich.LLMManager, error) {
	return productenrich.NewLLMManagerAdapter(cfg)
}

// newWebScraper 创建基于 1688 爬虫的 WebScraper（委托给 productenrich 包）。
func newWebScraper(cfg *config.Config) productenrich.WebScraper {
	return productenrichenrich.NewCrawler1688Adapter(cfg)
}

// poolSubmitter 将 worker.WorkerPool 适配为 productenrich.TaskSubmitter。
// 解耦 ProductService 与 WorkerPool 的双向依赖：Service 只感知提交能力，不感知 Pool 生命周期。
type poolSubmitter struct {
	pool worker.WorkerPool
}

func (s *poolSubmitter) Submit(taskID string) error {
	return s.pool.Submit(worker.WorkerJob{TaskData: taskID})
}

// newRedisClient 创建真实 Redis 客户端（连接失败时返回错误）。
func newRedisClient(cfg *config.RedisConfig, logger *logrus.Logger) (productenrich.RedisClient, error) {
	rc, err := redisclient.New(cfg)
	if err != nil {
		return nil, err
	}
	logger.Infof("Redis 已连接: %s:%d db=%d", cfg.Host, cfg.Port, cfg.DB)
	return rc, nil
}

// newDBTaskRepository 创建基于 PostgreSQL 的 TaskRepository，并自动迁移表结构。
// 返回的 closer 用于在服务退出时关闭数据库连接。
func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productenrich.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("数据库连接失败 (%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("数据库已连接: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
		return nil, nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	repo := store.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBImageTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productimage.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("鏁版嵁搴撹繛鎺ュけ璐?(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("productimage 鏁版嵁搴撳凡杩炴帴: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		return nil, nil, fmt.Errorf("productimage 鏁版嵁搴撹縼绉诲け璐? %w", err)
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}
