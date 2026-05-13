package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/tenantctx"
)

type MemTaskRepository struct {
	mu               sync.RWMutex
	tasks            map[string]*listingkit.Task
	canonicalProduct map[string]*listingkit.CanonicalProductCacheEntry
}

func NewMemTaskRepository() listingkit.Repository {
	return &MemTaskRepository{
		tasks:            make(map[string]*listingkit.Task),
		canonicalProduct: make(map[string]*listingkit.CanonicalProductCacheEntry),
	}
}

func (r *MemTaskRepository) CreateTask(ctx context.Context, task *listingkit.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.TenantID == "" {
		task.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if task.Request != nil && task.Request.TenantID == "" {
		task.Request.TenantID = task.TenantID
	}
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *MemTaskRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return nil, listingkit.ErrTaskNotFound
	}
	copied := *task
	return &copied, nil
}

func (r *MemTaskRepository) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	page, pageSize := normalizeTaskListPage(query)
	items := r.collectFilteredTasksLocked(ctx, query)

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

func (r *MemTaskRepository) ListTaskSummaryTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.collectFilteredTasksLocked(ctx, query), nil
}

func (r *MemTaskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
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

func (r *MemTaskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return listingkit.ErrTaskNotFound
	}
	task.Status = listingkit.TaskStatusFailed
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return listingkit.ErrTaskNotFound
	}
	task.Status = listingkit.TaskStatusPending
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return listingkit.ErrTaskNotFound
	}
	task.RetryCount++
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) SaveTaskResult(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return listingkit.ErrTaskNotFound
	}
	task.Result = result
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) MutateTaskResult(ctx context.Context, taskID string, mutate listingkit.TaskResultMutation) (*listingkit.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return nil, listingkit.ErrTaskNotFound
	}
	copied := *task
	out := &copied
	if mutate != nil {
		if err := mutate(task); err != nil {
			return out, err
		}
	}
	task.UpdatedAt = time.Now()
	copied = *task
	return &copied, nil
}

func (r *MemTaskRepository) GetCanonicalProductCache(ctx context.Context, fingerprint string) (*canonical.Product, error) {
	if fingerprint == "" {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.canonicalProduct == nil {
		return nil, nil
	}
	entry := r.canonicalProduct[canonicalCacheKey(ctx, fingerprint)]
	if entry == nil {
		return nil, nil
	}
	return entry.CanonicalProduct()
}

func (r *MemTaskRepository) SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *canonical.Product, sourceTaskID string) error {
	entry, err := listingkit.NewCanonicalProductCacheEntry(fingerprint, product, sourceTaskID)
	if err != nil {
		return err
	}
	entry.TenantID = tenantctx.TenantIDFromContext(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.canonicalProduct == nil {
		r.canonicalProduct = make(map[string]*listingkit.CanonicalProductCacheEntry)
	}
	r.canonicalProduct[canonicalCacheKey(ctx, fingerprint)] = entry
	return nil
}

func matchesTenantScope(ctx context.Context, recordTenantID string) bool {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if !ok {
		return true
	}
	return tenantctx.MatchesTenant(recordTenantID, tenantID)
}

func canonicalCacheKey(ctx context.Context, fingerprint string) string {
	return tenantctx.TenantIDFromContext(ctx) + ":" + fingerprint
}

func (r *MemTaskRepository) collectFilteredTasksLocked(ctx context.Context, query *listingkit.TaskListQuery) []listingkit.Task {
	items := make([]listingkit.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !matchesTenantScope(ctx, task.TenantID) {
			continue
		}
		if !listingkit.TaskMatchesListQuery(task, query) {
			continue
		}
		copied := *task
		items = append(items, copied)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}
