package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/database"
	"task-processor/internal/productenrich"
)

// =============================================================================
// LLM 适配器
// =============================================================================

type llmClientAdapter struct {
	client *openai.Client
}

func (a *llmClientAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.client.Generate(ctx, prompt)
}

func (a *llmClientAdapter) AnalyzeImage(ctx context.Context, imageURL string, prompt string) (string, error) {
	// 将图片 URL 嵌入 prompt，让文本模型基于 URL 描述进行分析
	// 若后续接入 vision 模型，替换此处实现即可
	fullPrompt := fmt.Sprintf("%s\n\nImage URL: %s", prompt, imageURL)
	return a.client.Generate(ctx, fullPrompt)
}

type llmManagerAdapter struct {
	manager *openai.Manager
}

func (a *llmManagerAdapter) GetClient(clientName string) (productenrich.LLMClient, error) {
	c, err := a.manager.GetClient(clientName)
	if err != nil {
		return nil, err
	}
	return &llmClientAdapter{client: c}, nil
}

func (a *llmManagerAdapter) GetDefaultClient() productenrich.LLMClient {
	return &llmClientAdapter{client: a.manager.GetDefaultClient()}
}

func newLLMManager(cfg config.OpenAIConfig) (productenrich.LLMManager, error) {
	// 构建客户端配置 map，先用顶层默认配置注册 "default"
	clientCfgs := map[string]*openai.ClientConfig{
		"default": openai.NewClientConfig(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.Timeout),
	}

	// 注册各阶段命名客户端（vision / fast / scorer 等）
	for name, c := range cfg.Clients {
		apiKey := c.APIKey
		if apiKey == "" {
			apiKey = cfg.APIKey // 继承默认 key
		}
		baseURL := c.BaseURL
		if baseURL == "" {
			baseURL = cfg.BaseURL
		}
		timeout := c.Timeout
		if timeout == 0 {
			timeout = cfg.Timeout
		}
		clientCfgs[name] = openai.NewClientConfig(apiKey, c.Model, baseURL, timeout)
	}

	mgr, err := openai.NewManager(&openai.ManagerConfig{
		Clients:       clientCfgs,
		DefaultClient: "default",
	})
	if err != nil {
		return nil, fmt.Errorf("创建 OpenAI Manager 失败: %w", err)
	}
	return &llmManagerAdapter{manager: mgr}, nil
}

// =============================================================================
// Redis 适配器（真实实现）
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
// Database TaskRepository（真实实现）
// =============================================================================

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productenrich.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewDatabase(&database.DatabaseConfig{
		Host:               cfg.Host,
		Port:               cfg.Port,
		User:               cfg.User,
		Password:           cfg.Password,
		Database:           cfg.Database,
		MaxConnections:     cfg.MaxConnections,
		MaxIdleConnections: cfg.MaxIdleConnections,
		ConnMaxLifetime:    cfg.ConnectionMaxLifetime,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("数据库连接失败 (%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("数据库已连接: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	// 自动迁移 Task 表结构
	if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
		return nil, nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	repo := productenrich.NewTaskRepository(db)
	closer := func() error { return database.CloseDatabase(db) }
	return repo, closer, nil
}

// =============================================================================
// 内存回退实现（database/redis 未配置时使用）
// =============================================================================

type memRedisEntry struct {
	value     string
	expiresAt time.Time
}

type memRedisClient struct {
	mu    sync.RWMutex
	store map[string]memRedisEntry
	lists map[string][]string
}

func newMemRedisClient() productenrich.RedisClient {
	return &memRedisClient{
		store: make(map[string]memRedisEntry),
		lists: make(map[string][]string),
	}
}

func (r *memRedisClient) Push(_ context.Context, key string, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists[key] = append(r.lists[key], value)
	return nil
}

func (r *memRedisClient) Get(_ context.Context, key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.store[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return "", fmt.Errorf("key expired: %s", key)
	}
	return entry.value, nil
}

func (r *memRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := memRedisEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	r.store[key] = entry
	return nil
}

func (r *memRedisClient) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, key)
	delete(r.lists, key)
	return nil
}

type memTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*productenrich.Task
}

func newMemTaskRepository() productenrich.TaskRepository {
	return &memTaskRepository{
		tasks: make(map[string]*productenrich.Task),
	}
}

func (r *memTaskRepository) CreateTask(_ context.Context, task *productenrich.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

func (r *memTaskRepository) GetTask(_ context.Context, taskID string) (*productenrich.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, productenrich.ErrTaskNotFound
	}
	return task, nil
}

func (r *memTaskRepository) UpdateTaskStatus(_ context.Context, taskID string, status productenrich.TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = status
	return nil
}

func (r *memTaskRepository) UpdateTaskError(_ context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = productenrich.TaskStatusFailed
	task.Error = errorMsg
	return nil
}

func (r *memTaskRepository) SaveTaskResult(_ context.Context, taskID string, result *productenrich.ProductJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = productenrich.TaskStatusCompleted
	task.Result = result
	return nil
}

func (r *memTaskRepository) IncrementRetryCount(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.RetryCount++
	return nil
}

func (r *memTaskRepository) ResetForRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = productenrich.TaskStatusPending
	// 保留 error 字段，不清空
	return nil
}

// =============================================================================
// WebScraper 适配器（将 Alibaba1688Processor 适配为 productenrich.WebScraper）
// =============================================================================

// crawler1688Adapter 将 alibaba1688.Alibaba1688Processor 适配为 productenrich.WebScraper。
type crawler1688Adapter struct {
	processor *alibaba1688.Alibaba1688Processor
}

// newWebScraper 创建基于 1688 爬虫的 WebScraper。
func newWebScraper(cfg *config.Config) productenrich.WebScraper {
	return &crawler1688Adapter{
		processor: alibaba1688.NewAlibaba1688Processor(cfg),
	}
}

// Scrape 实现 productenrich.WebScraper 接口，将 Product1688 转换为 ScrapedData。
func (a *crawler1688Adapter) Scrape(_ context.Context, url string) (*productenrich.ScrapedData, error) {
	product, err := a.processor.Process(url)
	if err != nil {
		return nil, fmt.Errorf("1688 scrape failed: %w", err)
	}

	specs := make(map[string]string, len(product.Specifications))
	for _, s := range product.Specifications {
		specs[s.Name] = s.Value
	}

	// 取最低价作为代表价格
	price := product.MinPrice

	return &productenrich.ScrapedData{
		Title:       product.Title,
		Description: a.buildDescription(product),
		Images:      product.Images,
		Price:       price,
		Specs:       specs,
	}, nil
}

// buildDescription 将 ProductDetails 拼接为描述文本。
func (a *crawler1688Adapter) buildDescription(product *alibaba1688model.Product1688) string {
	if len(product.ProductDetails) == 0 {
		return product.Title
	}
	var sb strings.Builder
	for _, d := range product.ProductDetails {
		if d.Content != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(d.Content)
		}
	}
	if sb.Len() == 0 {
		return product.Title
	}
	return sb.String()
}
