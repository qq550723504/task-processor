package store

import (
	"context"
	"fmt"
	"sync"

	"task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*productenrich.Task
}

func NewMemTaskRepository() productenrich.TaskRepository {
	return &MemTaskRepository{tasks: make(map[string]*productenrich.Task)}
}

func (r *MemTaskRepository) CreateTask(ctx context.Context, task *productenrich.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if !aiidentity.TenantCanCreateTask(ctx, task.PersistedExecutionEnvelope) {
		return productenrich.ErrTaskNotFound
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *MemTaskRepository) GetTask(ctx context.Context, taskID string) (*productenrich.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[taskID]
	if !ok || !aiidentity.TenantMatchesContext(ctx, task.ExecutionTenantID) {
		return nil, productenrich.ErrTaskNotFound
	}

	cp := *task
	return &cp, nil
}

func (r *MemTaskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != productenrich.TaskStatusPending {
		return productenrich.ErrTaskNotPending
	}
	task.Status = productenrich.TaskStatusProcessing
	task.Error = ""
	return nil
}

func (r *MemTaskRepository) MarkCompleted(ctx context.Context, taskID string, result *productenrich.ProductJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = productenrich.TaskStatusCompleted
	task.Error = ""
	task.Result = result
	return nil
}

func (r *MemTaskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = productenrich.TaskStatusFailed
	task.Error = errorMsg
	return nil
}

func (r *MemTaskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = productenrich.TaskStatusPending
	task.Error = ""
	return nil
}

func (r *MemTaskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status productenrich.TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = status
	return nil
}

func (r *MemTaskRepository) UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error {
	return r.MarkFailed(ctx, taskID, errorMsg)
}

func (r *MemTaskRepository) SaveTaskResult(ctx context.Context, taskID string, result *productenrich.ProductJSON) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = productenrich.TaskStatusCompleted
	task.Result = result
	return nil
}

func (r *MemTaskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.RetryCount++
	return nil
}

func (r *MemTaskRepository) ResetForRetry(ctx context.Context, taskID string) error {
	return r.PrepareRetry(ctx, taskID)
}

// taskForUpdate must be called while r.mu is write-locked.
func (r *MemTaskRepository) taskForUpdate(ctx context.Context, taskID string) (*productenrich.Task, error) {
	task, ok := r.tasks[taskID]
	if !ok || !aiidentity.TenantMatchesContext(ctx, task.ExecutionTenantID) {
		return nil, productenrich.ErrTaskNotFound
	}
	return task, nil
}
