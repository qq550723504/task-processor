package store

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/listingkit"
)

type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*listingkit.Task
}

func NewMemTaskRepository() listingkit.Repository {
	return &MemTaskRepository{tasks: make(map[string]*listingkit.Task)}
}

func (r *MemTaskRepository) CreateTask(_ context.Context, task *listingkit.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *MemTaskRepository) GetTask(_ context.Context, taskID string) (*listingkit.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, listingkit.ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *MemTaskRepository) MarkProcessing(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return listingkit.ErrTaskNotFound
	}
	if task.Status != listingkit.TaskStatusPending {
		return listingkit.ErrTaskNotPending
	}
	task.Status = listingkit.TaskStatusProcessing
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) MarkCompleted(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	task := r.tasks[taskID]
	task.Status = listingkit.TaskStatusCompleted
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return listingkit.ErrTaskNotFound
	}
	task.Status = listingkit.TaskStatusFailed
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) PrepareRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return listingkit.ErrTaskNotFound
	}
	task.Status = listingkit.TaskStatusPending
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) IncrementRetryCount(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return listingkit.ErrTaskNotFound
	}
	task.RetryCount++
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) SaveTaskResult(_ context.Context, taskID string, result *listingkit.ListingKitResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return listingkit.ErrTaskNotFound
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}
