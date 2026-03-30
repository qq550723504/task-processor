package store

import (
	"context"
	"sync"
	"time"

	productimage "task-processor/internal/productimage"
)

type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*productimage.Task
}

func NewMemTaskRepository() productimage.TaskRepository {
	return &MemTaskRepository{tasks: make(map[string]*productimage.Task)}
}

func (r *MemTaskRepository) CreateTask(_ context.Context, task *productimage.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *MemTaskRepository) GetTask(_ context.Context, taskID string) (*productimage.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return nil, productimage.ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *MemTaskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	if task.Status != productimage.TaskStatusPending {
		return productimage.ErrTaskNotPending
	}
	task.Status = productimage.TaskStatusProcessing
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) MarkCompleted(ctx context.Context, taskID string, result *productimage.ImageProcessResult) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	if err := r.UpdateTaskStatus(ctx, taskID, productimage.TaskStatusCompleted); err != nil {
		return err
	}
	return r.UpdateTaskMessage(ctx, taskID, "")
}

func (r *MemTaskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *productimage.ImageProcessResult, reason string) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	if err := r.UpdateTaskStatus(ctx, taskID, productimage.TaskStatusNeedsReview); err != nil {
		return err
	}
	return r.UpdateTaskMessage(ctx, taskID, reason)
}

func (r *MemTaskRepository) MarkRejected(ctx context.Context, taskID string, reason string) error {
	if err := r.UpdateTaskStatus(ctx, taskID, productimage.TaskStatusRejected); err != nil {
		return err
	}
	return r.UpdateTaskMessage(ctx, taskID, reason)
}

func (r *MemTaskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.UpdateTaskError(ctx, taskID, errorMsg)
}

func (r *MemTaskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	return r.ResetForRetry(ctx, taskID)
}

func (r *MemTaskRepository) UpdateTaskStatus(_ context.Context, taskID string, status productimage.TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) UpdateTaskError(_ context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	task.Status = productimage.TaskStatusFailed
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) UpdateTaskMessage(_ context.Context, taskID string, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	task.Error = reason
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) SaveTaskResult(_ context.Context, taskID string, result *productimage.ImageProcessResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) IncrementRetryCount(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	task.RetryCount++
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) ResetForRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok {
		return productimage.ErrTaskNotFound
	}
	task.Status = productimage.TaskStatusPending
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}
