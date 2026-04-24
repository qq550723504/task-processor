package store

import (
	"context"
	"sort"
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

func (r *MemTaskRepository) ListTasks(_ context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	page, pageSize := normalizeTaskListPage(query)
	items := make([]listingkit.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if query != nil && query.Status != "" && string(task.Status) != query.Status {
			continue
		}
		if query != nil && query.Platform != "" && !taskHasPlatform(task, query.Platform) {
			continue
		}
		copied := *task
		items = append(items, copied)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := int64(len(items))
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []listingkit.Task{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
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

func normalizeTaskListPage(query *listingkit.TaskListQuery) (int, int) {
	page := 1
	pageSize := 20
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func taskHasPlatform(task *listingkit.Task, platform string) bool {
	if task == nil || task.Request == nil {
		return false
	}
	for _, candidate := range task.Request.Platforms {
		if candidate == platform {
			return true
		}
	}
	return false
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

func (r *MemTaskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *listingkit.ListingKitResult, reason string) error {
	if err := r.SaveTaskResult(ctx, taskID, result); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	task := r.tasks[taskID]
	task.Status = listingkit.TaskStatusNeedsReview
	task.Error = reason
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
