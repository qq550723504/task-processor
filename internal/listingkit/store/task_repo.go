package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/tenantctx"
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
		if task.Request != nil && task.Request.TenantID == "" {
			task.Request.TenantID = task.TenantID
		}
	}
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	var task listingkit.Task
	db := applyTenantScope(r.db.WithContext(ctx), ctx, "tenant_id")
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, listingkit.ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, int64, error) {
	page, pageSize := normalizeTaskListPage(query)
	db := applyTenantScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx, "tenant_id")
	if query != nil && query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}

	if query != nil && query.Platform != "" {
		var all []listingkit.Task
		if err := db.Order("created_at DESC").Find(&all).Error; err != nil {
			return nil, 0, err
		}
		filtered := make([]listingkit.Task, 0, len(all))
		for i := range all {
			if taskHasPlatform(&all[i], query.Platform) {
				filtered = append(filtered, all[i])
			}
		}
		total := int64(len(filtered))
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

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tasks []listingkit.Task
	if err := db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (r *taskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	result := r.db.WithContext(ctx).
		Model(&listingkit.Task{}).
		Scopes(tenantScope(ctx, "tenant_id")).
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
	return r.db.WithContext(ctx).Model(&listingkit.Task{}).Scopes(tenantScope(ctx, "tenant_id")).Where("id = ?", taskID).UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).Error
}

func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result})
}

func (r *taskRepository) MutateTaskResult(ctx context.Context, taskID string, mutate listingkit.TaskResultMutation) (*listingkit.Task, error) {
	var out *listingkit.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := applyTenantScope(tx.Clauses(clause.Locking{Strength: "UPDATE"}), ctx, "tenant_id").Where("id = ?", taskID).First(&task).Error; err != nil {
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
			Scopes(tenantScope(ctx, "tenant_id")).
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

func (r *taskRepository) updateTaskFields(ctx context.Context, taskID string, updates map[string]any) error {
	updates["updated_at"] = gorm.Expr("NOW()")
	result := r.db.WithContext(ctx).Model(&listingkit.Task{}).Scopes(tenantScope(ctx, "tenant_id")).Where("id = ?", taskID).Updates(updates)
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

func storedCanonicalFingerprint(ctx context.Context, fingerprint string) string {
	tenantID := tenantctx.TenantIDFromContext(ctx)
	if tenantID == tenantctx.DefaultTenantID {
		return fingerprint
	}
	return tenantID + ":" + fingerprint
}
