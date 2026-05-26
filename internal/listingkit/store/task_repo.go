package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/tenantctx"
	sheinpub "task-processor/internal/publishing/shein"
)

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) listingkit.Repository {
	return &taskRepository{db: db}
}

func (r *taskRepository) CreateTask(ctx context.Context, task *listingkit.Task) error {
	if task != nil {
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
	}
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	var task listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx), ctx)
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, listingkit.ErrTaskNotFound
		}
		return nil, err
	}
	if !taskVisibleToUser(ctx, &task) {
		return nil, listingkit.ErrTaskNotFound
	}
	return &task, nil
}

func (r *taskRepository) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, int64, error) {
	page, pageSize := normalizeTaskListPage(query)
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if query != nil && query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if query != nil && (query.Platform != "" || query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "") {
		var candidates []taskListFilterRow
		columns := []string{"id", "created_at", "status", "user_id"}
		if query.Platform != "" || query.SheinWorkQueue != "" {
			columns = append(columns, "request")
		}
		if query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "" {
			columns = append(columns, "result")
		}
		if err := db.Select(columns).Order("created_at DESC").Find(&candidates).Error; err != nil {
			return nil, 0, err
		}
		filteredIDs := make([]string, 0, len(candidates))
		for i := range candidates {
			if !taskVisibleToUser(ctx, &listingkit.Task{UserID: candidates[i].UserID, Request: &listingkit.GenerateRequest{UserID: candidates[i].RequestUserID}}) {
				continue
			}
			if !matchesTaskListFilterRow(&candidates[i], query) {
				continue
			}
			filteredIDs = append(filteredIDs, candidates[i].ID)
		}
		total := int64(len(filteredIDs))
		start := (page - 1) * pageSize
		if start >= len(filteredIDs) {
			return []listingkit.Task{}, total, nil
		}
		end := start + pageSize
		if end > len(filteredIDs) {
			end = len(filteredIDs)
		}
		return r.loadTasksByIDs(ctx, filteredIDs[start:end], total)
	}

	var total int64
	if !listingkit.OwnerScopeEnabled() || listingkit.RequestUserIDFromContext(ctx) == "" {
		if err := db.Count(&total).Error; err != nil {
			return nil, 0, err
		}
		var tasks []listingkit.Task
		if err := db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
			return nil, 0, err
		}
		return tasks, total, nil
	}
	var tasks []listingkit.Task
	if err := db.Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	filtered := filterTasksForUser(ctx, tasks)
	total = int64(len(filtered))
	start := (page - 1) * pageSize
	if start >= len(filtered) {
		return []listingkit.Task{}, total, nil
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}

func (r *taskRepository) ListTaskSummaryTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, error) {
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if query != nil && query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if query != nil && (query.Platform != "" || query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "") {
		var candidates []taskListFilterRow
		columns := []string{"id", "created_at", "status", "user_id"}
		if query.Platform != "" || query.SheinWorkQueue != "" {
			columns = append(columns, "request")
		}
		if query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "" {
			columns = append(columns, "result")
		}
		if err := db.Select(columns).Order("created_at DESC").Find(&candidates).Error; err != nil {
			return nil, err
		}
		filteredIDs := make([]string, 0, len(candidates))
		for i := range candidates {
			if !taskVisibleToUser(ctx, &listingkit.Task{UserID: candidates[i].UserID, Request: &listingkit.GenerateRequest{UserID: candidates[i].RequestUserID}}) {
				continue
			}
			if !matchesTaskListFilterRow(&candidates[i], query) {
				continue
			}
			filteredIDs = append(filteredIDs, candidates[i].ID)
		}
		tasks, _, err := r.loadTasksByIDs(ctx, filteredIDs, int64(len(filteredIDs)))
		return tasks, err
	}

	var tasks []listingkit.Task
	if err := db.Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return filterTasksForUser(ctx, tasks), nil
}

type taskListFilterRow struct {
	ID            string    `gorm:"column:id"`
	UserID        string    `gorm:"column:user_id"`
	Status        string    `gorm:"column:status"`
	Request       string    `gorm:"column:request"`
	Result        string    `gorm:"column:result"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	RequestUserID string    `gorm:"-"`
}

type taskListFilterRequest struct {
	Platforms []string `json:"platforms,omitempty"`
	UserID    string   `json:"user_id,omitempty"`
}

type taskListFilterResult struct {
	Shein *sheinpub.Package `json:"shein,omitempty"`
}

func matchesTaskListFilterRow(row *taskListFilterRow, query *listingkit.TaskListQuery) bool {
	if row == nil {
		return false
	}
	task := &listingkit.Task{}
	task.Status = listingkit.TaskStatus(row.Status)
	if query != nil && (query.Platform != "" || query.SheinWorkQueue != "") {
		var request taskListFilterRequest
		if err := json.Unmarshal([]byte(row.Request), &request); err == nil {
			task.Request = &listingkit.GenerateRequest{Platforms: request.Platforms, UserID: request.UserID}
			row.RequestUserID = request.UserID
		}
	}
	if task.Request == nil {
		task.Request = &listingkit.GenerateRequest{UserID: row.RequestUserID}
	}
	if query != nil && (query.SheinWorkflowStatus != "" || query.SheinBlockerKey != "" || query.SheinWarningKey != "" || query.SheinWorkQueue != "" || query.SheinActionQueue != "") {
		var result taskListFilterResult
		if err := json.Unmarshal([]byte(row.Result), &result); err == nil {
			task.Result = &listingkit.ListingKitResult{Shein: result.Shein}
		}
	}
	return listingkit.TaskMatchesListQuery(task, query)
}

func (r *taskRepository) loadTasksByIDs(ctx context.Context, ids []string, total int64) ([]listingkit.Task, int64, error) {
	if len(ids) == 0 {
		return []listingkit.Task{}, total, nil
	}
	var tasks []listingkit.Task
	db := applyTaskAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	if err := db.Where("id IN ?", ids).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	tasks = filterTasksForUser(ctx, tasks)
	order := make(map[string]int, len(ids))
	for i, id := range ids {
		order[id] = i
	}
	ordered := make([]listingkit.Task, 0, len(tasks))
	for _, task := range tasks {
		index, ok := order[task.ID]
		if !ok {
			continue
		}
		if len(ordered) <= index {
			next := make([]listingkit.Task, index+1)
			copy(next, ordered)
			ordered = next
		}
		ordered[index] = task
	}
	compacted := make([]listingkit.Task, 0, len(tasks))
	for _, task := range ordered {
		if task.ID == "" {
			continue
		}
		compacted = append(compacted, task)
	}
	return compacted, total, nil
}

func (r *taskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	result := r.db.WithContext(ctx).
		Model(&listingkit.Task{}).
		Scopes(taskAccessScope(ctx)).
		Where("id = ? AND status = ?", taskID, listingkit.TaskStatusPending).
		Updates(map[string]any{
			"status":     listingkit.TaskStatusProcessing,
			"updated_at": gorm.Expr("NOW()"),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	task, err := r.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != listingkit.TaskStatusPending {
		return listingkit.ErrTaskNotPending
	}
	return listingkit.ErrTaskNotFound
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
		"status": listingkit.TaskStatusCompleted,
		"error":  "",
	})
}

func (r *taskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *listingkit.ListingKitResult, reason string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
		"status": listingkit.TaskStatusNeedsReview,
		"error":  reason,
	})
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": listingkit.TaskStatusFailed,
		"error":  errorMsg,
	})
}

func (r *taskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": listingkit.TaskStatusPending,
		"error":  "",
	})
}

func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	return r.db.WithContext(ctx).Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ?", taskID).UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).Error
}

func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result})
}

func (r *taskRepository) MutateTaskResult(ctx context.Context, taskID string, mutate listingkit.TaskResultMutation) (*listingkit.Task, error) {
	var out *listingkit.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTaskAccessScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return listingkit.ErrTaskNotFound
			}
			return err
		}
		copied := task
		out = &copied
		if mutate != nil {
			if err := mutate(&task); err != nil {
				return err
			}
		}
		task.UpdatedAt = time.Now()
		if err := tx.Model(&listingkit.Task{}).
			Scopes(taskAccessScope(ctx)).
			Where("id = ?", taskID).
			Updates(map[string]any{
				"result":     task.Result,
				"updated_at": gorm.Expr("NOW()"),
			}).Error; err != nil {
			return fmt.Errorf("failed to update task result: %w", err)
		}
		copied = task
		out = &copied
		return nil
	})
	return out, err
}

func (r *taskRepository) GetCanonicalProductCache(ctx context.Context, fingerprint string) (*canonical.Product, error) {
	if fingerprint == "" {
		return nil, nil
	}
	var entry listingkit.CanonicalProductCacheEntry
	db := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id")
	if err := db.Where("fingerprint = ?", storedCanonicalFingerprint(ctx, fingerprint)).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return entry.CanonicalProduct()
}

func (r *taskRepository) SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *canonical.Product, sourceTaskID string) error {
	entry, err := listingkit.NewCanonicalProductCacheEntry(fingerprint, product, sourceTaskID)
	if err != nil {
		return err
	}
	entry.TenantID = tenantctx.TenantIDFromContext(ctx)
	entry.Fingerprint = storedCanonicalFingerprint(ctx, fingerprint)
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "fingerprint"}},
			DoUpdates: clause.Assignments(map[string]any{
				"product":        entry.Product,
				"tenant_id":      entry.TenantID,
				"source_task_id": sourceTaskID,
				"updated_at":     gorm.Expr("NOW()"),
			}),
		}).
		Create(entry).Error
}

func (r *taskRepository) GetSDSBaselineCache(ctx context.Context, tenantID, baselineKey string) (*listingkit.SDSBaselineCacheEntry, error) {
	resolvedTenantID, logicalKey, storedKey, err := listingkit.ResolveSDSBaselineCacheScope(ctx, tenantID, baselineKey)
	if err != nil {
		return nil, err
	}
	if storedKey == "" {
		return nil, nil
	}
	var entry listingkit.SDSBaselineCacheEntry
	db := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id")
	if err := db.Where("baseline_key = ?", storedKey).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	entry.TenantID = resolvedTenantID
	entry.BaselineKey = logicalKey
	return &entry, nil
}

func (r *taskRepository) SaveSDSBaselineCache(ctx context.Context, entry *listingkit.SDSBaselineCacheEntry) error {
	if entry == nil {
		return nil
	}
	tenantID, _, storedKey, err := listingkit.ResolveSDSBaselineCacheScope(ctx, entry.TenantID, entry.BaselineKey)
	if err != nil {
		return err
	}
	if storedKey == "" {
		return nil
	}
	cloned, err := entry.Clone()
	if err != nil {
		return err
	}
	cloned.TenantID = tenantID
	cloned.BaselineKey = storedKey
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "baseline_key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"tenant_id":              cloned.TenantID,
				"status":                 cloned.Status,
				"version":                cloned.Version,
				"source_task_id":         cloned.SourceTaskID,
				"identity":               cloned.Identity,
				"canonical_product_base": cloned.CanonicalProductBase,
				"updated_at":             time.Now(),
			}),
		}).
		Create(cloned).Error
}

func (r *taskRepository) updateTaskFields(ctx context.Context, taskID string, updates map[string]any) error {
	updates["updated_at"] = gorm.Expr("NOW()")
	result := r.db.WithContext(ctx).Model(&listingkit.Task{}).Scopes(taskAccessScope(ctx)).Where("id = ?", taskID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return listingkit.ErrTaskNotFound
	}
	return nil
}

func tenantScope(ctx context.Context, column string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return applyTenantScope(db, ctx, column)
	}
}

func taskAccessScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return applyTaskAccessScope(db, ctx)
	}
}

func applyTenantScope(db *gorm.DB, ctx context.Context, column string) *gorm.DB {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if !ok {
		return db
	}
	if tenantID == tenantctx.DefaultTenantID {
		return db.Where("("+column+" = ? OR "+column+" = '' OR "+column+" IS NULL)", tenantID)
	}
	return db.Where(column+" = ?", tenantID)
}

func applyTaskAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	db = applyTenantScope(db, ctx, "tenant_id")
	if !listingkit.OwnerScopeEnabled() {
		return db
	}
	if listingkit.RequestHasPlatformAdminAccess(ctx) {
		return db
	}
	userID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
	if userID == "" {
		return db
	}
	return db.Where("user_id = ?", userID)
}

func storedCanonicalFingerprint(ctx context.Context, fingerprint string) string {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	if tenantID == tenantctx.DefaultTenantID {
		return fingerprint
	}
	return tenantID + ":" + fingerprint
}

func filterTasksForUser(ctx context.Context, tasks []listingkit.Task) []listingkit.Task {
	if !listingkit.OwnerScopeEnabled() {
		return tasks
	}
	userID := listingkit.RequestUserIDFromContext(ctx)
	if userID == "" {
		return tasks
	}
	filtered := make([]listingkit.Task, 0, len(tasks))
	for _, task := range tasks {
		if taskVisibleToUser(ctx, &task) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func taskVisibleToUser(ctx context.Context, task *listingkit.Task) bool {
	if !listingkit.OwnerScopeEnabled() {
		return true
	}
	if listingkit.RequestHasPlatformAdminAccess(ctx) {
		return true
	}
	requestUserID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
	if requestUserID == "" {
		return true
	}
	return strings.TrimSpace(listingkit.ResolveTaskUserID(task)) == requestUserID
}
