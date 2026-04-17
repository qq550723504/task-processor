package httpapi

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimagestore "task-processor/internal/productimage/store"
)

func newLLMManager(cfg config.OpenAIConfig) (productenrich.LLMManager, error) {
	return productenrich.NewLLMManagerAdapter(cfg)
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

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productenrich.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
		return nil, nil, fmt.Errorf("database auto-migrate failed: %w", err)
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
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		return nil, nil, fmt.Errorf("productimage auto-migrate failed: %w", err)
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBAmazonListingTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (amazonlisting.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&amazonlisting.Task{}); err != nil {
		return nil, nil, fmt.Errorf("amazonlisting auto-migrate failed: %w", err)
	}

	repo := amazonlistingstore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

func newDBListingKitTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (listingkit.Repository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&listingkit.Task{}); err != nil {
		return nil, nil, fmt.Errorf("listingkit auto-migrate failed: %w", err)
	}

	repo := listingkitstore.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}
