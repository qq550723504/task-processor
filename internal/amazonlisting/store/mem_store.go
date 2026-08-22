package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/shared/aiidentity"
)

type MemTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]*amazonlisting.Task
}

func NewMemTaskRepository() amazonlisting.Repository {
	return &MemTaskRepository{tasks: make(map[string]*amazonlisting.Task)}
}

func (r *MemTaskRepository) CreateTask(_ context.Context, task *amazonlisting.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *MemTaskRepository) GetTask(ctx context.Context, taskID string) (*amazonlisting.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok || !aiidentity.TenantMatchesContext(ctx, task.ExecutionTenantID) {
		return nil, amazonlisting.ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *MemTaskRepository) ListTasks(ctx context.Context, statuses []amazonlisting.TaskStatus, limit int) ([]*amazonlisting.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	allowed := make(map[amazonlisting.TaskStatus]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status] = struct{}{}
	}
	items := make([]*amazonlisting.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !aiidentity.TenantMatchesContext(ctx, task.ExecutionTenantID) {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[task.Status]; !ok {
				continue
			}
		}
		copied := *task
		items = append(items, &copied)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemTaskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != amazonlisting.TaskStatusPending {
		return amazonlisting.ErrTaskNotPending
	}
	task.Status = amazonlisting.TaskStatusProcessing
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) MarkCompleted(ctx context.Context, taskID string, result *amazonlisting.AmazonListingDraft) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	return r.UpdateTaskStatus(ctx, taskID, amazonlisting.TaskStatusCompleted)
}

func (r *MemTaskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *amazonlisting.AmazonListingDraft, reason string) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	if err := r.UpdateTaskStatus(ctx, taskID, amazonlisting.TaskStatusNeedsReview); err != nil {
		return err
	}
	return r.UpdateTaskError(ctx, taskID, reason)
}

func (r *MemTaskRepository) MarkRejected(ctx context.Context, taskID string, reason string) error {
	if err := r.UpdateTaskStatus(ctx, taskID, amazonlisting.TaskStatusRejected); err != nil {
		return err
	}
	return r.UpdateTaskError(ctx, taskID, reason)
}

func (r *MemTaskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.UpdateTaskError(ctx, taskID, errorMsg)
}

func (r *MemTaskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	return r.ResetForRetry(ctx, taskID)
}

func (r *MemTaskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.RetryCount++
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status amazonlisting.TaskStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) SaveTaskResult(ctx context.Context, taskID string, result *amazonlisting.AmazonListingDraft) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) ResetForRetry(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, err := r.taskForUpdate(ctx, taskID)
	if err != nil {
		return err
	}
	task.Status = amazonlisting.TaskStatusPending
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

// taskForUpdate must be called while r.mu is write-locked.
func (r *MemTaskRepository) taskForUpdate(ctx context.Context, taskID string) (*amazonlisting.Task, error) {
	task, ok := r.tasks[taskID]
	if !ok || !aiidentity.TenantMatchesContext(ctx, task.ExecutionTenantID) {
		return nil, amazonlisting.ErrTaskNotFound
	}
	return task, nil
}
