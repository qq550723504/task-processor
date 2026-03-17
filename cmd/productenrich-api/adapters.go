package main

import (
"context"
"fmt"
"sync"
"time"

goredis "github.com/redis/go-redis/v9"
"github.com/sirupsen/logrus"

"task-processor/internal/core/config"
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

func (a *llmClientAdapter) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
return "", fmt.Errorf("AnalyzeImage not implemented")
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
clientCfg := openai.NewClientConfig(cfg.APIKey, cfg.Model, cfg.BaseURL, cfg.Timeout)
mgr, err := openai.NewManager(&openai.ManagerConfig{
Clients:       map[string]*openai.ClientConfig{"default": clientCfg},
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
