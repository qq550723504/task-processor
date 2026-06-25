package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit"
	"task-processor/internal/shared/tenantctx"
)

type MemTaskRepository struct {
	mu               sync.RWMutex
	tasks            map[string]*listingkit.Task
	canonicalProduct map[string]*listingkit.CanonicalProductCacheEntry
	sdsBaselineCache map[string]*listingkit.SDSBaselineCacheEntry
}

func NewMemTaskRepository() listingkit.Repository {
	return &MemTaskRepository{
		tasks:            make(map[string]*listingkit.Task),
		canonicalProduct: make(map[string]*listingkit.CanonicalProductCacheEntry),
		sdsBaselineCache: make(map[string]*listingkit.SDSBaselineCacheEntry),
	}
}

func (r *MemTaskRepository) CreateTask(ctx context.Context, task *listingkit.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.TenantID == "" {
		task.TenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if task.UserID == "" {
		task.UserID = listingkit.ResolveTaskUserID(task)
	}
	if task.Request != nil && task.Request.TenantID == "" {
		task.Request.TenantID = task.TenantID
	}
	if task.Request != nil && task.Request.UserID == "" {
		task.Request.UserID = task.UserID
	}
	copied := *task
	r.tasks[task.ID] = &copied
	return nil
}

func (r *MemTaskRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) || !taskVisibleToUser(ctx, task) {
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

func (r *MemTaskRepository) ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	targets := normalizedSheinSourceSDSTargets(query)
	if query == nil || query.StoreID <= 0 || len(targets) == 0 {
		return []listingkit.SheinSourceSDSMetadataRecord{}, nil
	}
	tasks := make([]listingkit.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !taskVisibleToUser(ctx, task) {
			continue
		}
		if listingkit.RequestUserIDFromContext(ctx) == "" && !matchesTenantScope(ctx, task.TenantID) {
			continue
		}
		copied := *task
		tasks = append(tasks, copied)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	return collectSheinSourceSDSMetadata(tasks, query.StoreID, targets), nil
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

func (r *MemTaskRepository) MarkBlockedRetryable(ctx context.Context, taskID string, block *listingkit.RetryableBlock, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return listingkit.ErrTaskNotFound
	}
	task.Status = listingkit.TaskStatusBlockedRetryable
	task.RetryableBlock = copyRetryableBlock(block)
	task.Error = errorMsg
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) ListRecoverableTasks(ctx context.Context, query *listingkit.RecoverableTaskQuery) ([]listingkit.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dueBefore := time.Time{}
	if query != nil {
		dueBefore = query.DueBefore
	}
	items := make([]listingkit.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !matchesTenantScope(ctx, task.TenantID) {
			continue
		}
		if !taskIsRecoverable(task, dueBefore, false) {
			continue
		}
		copied := *task
		items = append(items, copied)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].RetryableBlock.NextRetryAt
		right := items[j].RetryableBlock.NextRetryAt
		switch {
		case left == nil && right == nil:
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		case left == nil:
			return false
		case right == nil:
			return true
		case !left.Equal(*right):
			return left.Before(*right)
		case !items[i].CreatedAt.Equal(items[j].CreatedAt):
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		default:
			return items[i].ID < items[j].ID
		}
	})
	limit := normalizeRecoverableTaskLimit(query)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *MemTaskRepository) RecoverBlockedTaskNow(ctx context.Context, taskID string, recoveredAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || !matchesTenantScope(ctx, task.TenantID) {
		return listingkit.ErrTaskNotFound
	}
	force := recoveredAt.IsZero()
	effectiveRecoveredAt := normalizeRecoverTimestamp(recoveredAt)
	if !taskIsRecoverable(task, effectiveRecoveredAt, force) {
		return listingkit.ErrTaskNotRecoverable
	}
	block := listingkit.BuildRecoveredRetryableBlock(task.RetryableBlock, effectiveRecoveredAt)
	task.Status = listingkit.TaskStatusPending
	task.RetryableBlock = block
	task.Error = ""
	task.UpdatedAt = time.Now()
	return nil
}

func (r *MemTaskRepository) BulkRecoverBlockedTasks(ctx context.Context, query *listingkit.RecoverBlockedTasksQuery) (int64, error) {
	listQuery := &listingkit.RecoverableTaskQuery{}
	if query != nil {
		listQuery.DueBefore = query.DueBefore
		listQuery.Limit = normalizeRecoverableTaskLimitFromValue(query.Limit)
	}
	items, err := r.ListRecoverableTasks(ctx, listQuery)
	if err != nil {
		return 0, err
	}
	recoverAt := time.Now().UTC()
	if query != nil && !query.RecoverAt.IsZero() {
		recoverAt = query.RecoverAt
	}
	recoverAt = normalizeRecoverTimestamp(recoverAt)
	var recovered int64
	for i := range items {
		if err := r.RecoverBlockedTaskNow(ctx, items[i].ID, recoverAt); err != nil {
			if err == listingkit.ErrTaskNotRecoverable {
				continue
			}
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
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

func (r *MemTaskRepository) GetSDSBaselineCache(ctx context.Context, tenantID, baselineKey string) (*listingkit.SDSBaselineCacheEntry, error) {
	resolvedTenantID, logicalKey, storageKey, err := listingkit.ResolveSDSBaselineCacheScope(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, err
	}
	if storageKey == "" {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.sdsBaselineCache == nil {
		return nil, nil
	}
	entry, err := cloneSDSBaselineCacheEntry(r.sdsBaselineCache[storageKey])
	if err != nil || entry == nil {
		return entry, err
	}
	entry.TenantID = resolvedTenantID
	entry.BaselineKey = logicalKey
	return entry, nil
}

func (r *MemTaskRepository) SaveSDSBaselineCache(ctx context.Context, entry *listingkit.SDSBaselineCacheEntry) error {
	if entry == nil {
		return nil
	}
	resolvedTenantID, logicalKey, storageKey, err := listingkit.ResolveSDSBaselineCacheScope(ctx, entry.TenantID, entry.BaselineKey)
	if err != nil {
		return err
	}
	cloned, err := cloneSDSBaselineCacheEntry(entry)
	if err != nil {
		return err
	}
	cloned.TenantID = resolvedTenantID
	cloned.BaselineKey = logicalKey
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sdsBaselineCache == nil {
		r.sdsBaselineCache = make(map[string]*listingkit.SDSBaselineCacheEntry)
	}
	r.sdsBaselineCache[storageKey] = cloned
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

func cloneSDSBaselineCacheEntry(entry *listingkit.SDSBaselineCacheEntry) (*listingkit.SDSBaselineCacheEntry, error) {
	return entry.Clone()
}

func (r *MemTaskRepository) collectFilteredTasksLocked(ctx context.Context, query *listingkit.TaskListQuery) []listingkit.Task {
	items := make([]listingkit.Task, 0, len(r.tasks))
	for _, task := range r.tasks {
		if !matchesTenantScope(ctx, task.TenantID) {
			continue
		}
		if !taskVisibleToUser(ctx, task) {
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
