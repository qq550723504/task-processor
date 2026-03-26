// Package productenrich 提供内存版 RedisClient 和 TaskRepository，
// 用于测试、本地调试及无外部依赖的降级场景。
package productenrich

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// 内存 RedisClient
// =============================================================================

type memRedisEntry struct {
	value     string
	expiresAt time.Time
}

// MemRedisClient 基于内存的 RedisClient 实现。
type MemRedisClient struct {
	mu    sync.RWMutex
	store map[string]memRedisEntry
	lists map[string][]string
}

// NewMemRedisClient 创建内存 RedisClient。
func NewMemRedisClient() RedisClient {
	return &MemRedisClient{
		store: make(map[string]memRedisEntry),
		lists: make(map[string][]string),
	}
}

func (r *MemRedisClient) Push(_ context.Context, key string, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lists[key] = append(r.lists[key], value)
	return nil
}

func (r *MemRedisClient) Get(_ context.Context, key string) (string, error) {
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

func (r *MemRedisClient) Set(_ context.Context, key string, value string, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := memRedisEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	r.store[key] = entry
	return nil
}

func (r *MemRedisClient) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.store, key)
	delete(r.lists, key)
	return nil
}

// =============================================================================
// 内存 TaskRepository
// =============================================================================

// MemTaskRepository 基于内存的 TaskRepository 实现。
type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewMemTaskRepository 创建内存 TaskRepository。
func NewMemTaskRepository() TaskRepository {
	return &MemTaskRepository{tasks: make(map[string]*Task)}
}

func (r *MemTaskRepository) CreateTask(_ context.Context, task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

func (r *MemTaskRepository) GetTask(_ context.Context, taskID string) (*Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	cp := *task
	return &cp, nil
}

func (r *MemTaskRepository) UpdateTaskStatus(_ context.Context, taskID string, status TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = status
	return nil
}

func (r *MemTaskRepository) UpdateTaskError(_ context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = TaskStatusFailed
	task.Error = errorMsg
	return nil
}

func (r *MemTaskRepository) SaveTaskResult(_ context.Context, taskID string, result *ProductJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = TaskStatusCompleted
	task.Result = result
	return nil
}

func (r *MemTaskRepository) IncrementRetryCount(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.RetryCount++
	return nil
}

func (r *MemTaskRepository) ResetForRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = TaskStatusPending
	return nil
}
