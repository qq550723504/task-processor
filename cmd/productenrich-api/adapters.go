package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
)

// =============================================================================
// LLM 适配器
// =============================================================================

// llmClientAdapter 将 openai.Client 适配为 productenrich.LLMClient
type llmClientAdapter struct {
	client *openai.Client
}

func (a *llmClientAdapter) Generate(ctx context.Context, prompt string) (string, error) {
	return a.client.Generate(ctx, prompt)
}

// AnalyzeImage 当前 openai.Client 不支持图片分析，返回未实现错误
func (a *llmClientAdapter) AnalyzeImage(_ context.Context, _ string, _ string) (string, error) {
	return "", fmt.Errorf("AnalyzeImage not implemented")
}

// llmManagerAdapter 将 openai.Manager 适配为 productenrich.LLMManager
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

// newLLMManager 从 openai 配置创建 LLMManager
func newLLMManager(apiKey, model, baseURL string, timeoutSec int) (productenrich.LLMManager, error) {
	clientCfg := openai.NewClientConfig(apiKey, model, baseURL, timeoutSec)
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
// 内存 Redis 适配器（无 redis 依赖时的替代实现）
// =============================================================================

type memRedisEntry struct {
	value     string
	expiresAt time.Time
}

// memRedisClient 基于内存的 RedisClient 实现，适用于单实例开发/测试场景
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

// =============================================================================
// 内存 TaskRepository 适配器
// =============================================================================

// memTaskRepository 基于内存的 TaskRepository，适用于无数据库场景
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
