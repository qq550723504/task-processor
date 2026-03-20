package main

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/database"
	"task-processor/internal/productenrich"
)

// newLLMManager 创建 OpenAI LLMManager（委托给 internal/productenrich）。
func newLLMManager(cfg config.OpenAIConfig) (productenrich.LLMManager, error) {
	return productenrich.NewLLMManagerAdapter(cfg)
}

// newWebScraper 创建基于 1688 爬虫的 WebScraper（委托给 internal/productenrich）。
func newWebScraper(cfg *config.Config) productenrich.WebScraper {
	return productenrich.NewCrawler1688Adapter(cfg)
}

// newMemRedisClient 创建内存 RedisClient（委托给 internal/productenrich）。
func newMemRedisClient() productenrich.RedisClient {
	return productenrich.NewMemRedisClient()
}

// newMemTaskRepository 创建内存 TaskRepository（委托给 internal/productenrich）。
func newMemTaskRepository() productenrich.TaskRepository {
	return productenrich.NewMemTaskRepository()
}

// =============================================================================
// Redis 真实实现（仅 productenrich-api 需要）
// =============================================================================

type redisClient struct {
	rdb *goredis.Client
}

func newRedisClient(cfg *config.RedisConfig, logger *logrus.Logger) (productenrich.RedisClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is nil")
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis 连接失败 (%s:%d): %w", cfg.Host, cfg.Port, err)
	}
	logger.Infof("Redis 已连接: %s:%d db=%d", cfg.Host, cfg.Port, cfg.DB)
	return &redisClient{rdb: rdb}, nil
}

func (r *redisClient) Push(ctx context.Context, key string, value string) error {
	return r.rdb.RPush(ctx, key, value).Err()
}

func (r *redisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.rdb.Get(ctx, key).Result()
	if err == goredis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return val, err
}

func (r *redisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.rdb.Set(ctx, key, value, ttl).Err()
}

func (r *redisClient) Delete(ctx context.Context, key string) error {
	return r.rdb.Del(ctx, key).Err()
}

// =============================================================================
// Database TaskRepository 真实实现（仅 productenrich-api 需要）
// =============================================================================

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

	repo := productenrich.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}
