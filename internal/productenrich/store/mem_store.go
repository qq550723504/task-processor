package store

import (
	"context"
	"fmt"
	"sync"

	"task-processor/internal/productenrich"
)

type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*productenrich.Task
}

func NewMemTaskRepository() productenrich.TaskRepository {
	return &MemTaskRepository{tasks: make(map[string]*productenrich.Task)}
}

func (r *MemTaskRepository) CreateTask(_ context.Context, task *productenrich.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

func (r *MemTaskRepository) GetTask(_ context.Context, taskID string) (*productenrich.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[taskID]
	if !ok {
		return nil, productenrich.ErrTaskNotFound
	}

	cp := *task
	return &cp, nil
}

func (r *MemTaskRepository) MarkProcessing(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = productenrich.TaskStatusProcessing
	task.Error = ""
	return nil
}

func (r *MemTaskRepository) MarkCompleted(_ context.Context, taskID string, result *productenrich.ProductJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = productenrich.TaskStatusCompleted
	task.Error = ""
	task.Result = result
	return nil
}

func (r *MemTaskRepository) MarkFailed(_ context.Context, taskID string, errorMsg string) error {
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

func (r *MemTaskRepository) PrepareRetry(_ context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, ok := r.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = productenrich.TaskStatusPending
	task.Error = ""
	return nil
}

func (r *MemTaskRepository) UpdateTaskStatus(_ context.Context, taskID string, status productenrich.TaskStatus) error {
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
	return r.MarkFailed(context.Background(), taskID, errorMsg)
}

func (r *MemTaskRepository) SaveTaskResult(_ context.Context, taskID string, result *productenrich.ProductJSON) error {
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
	return r.PrepareRetry(context.Background(), taskID)
}
