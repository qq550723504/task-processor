package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
)

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) listingkit.Repository {
	return &taskRepository{db: db}
}

func (r *taskRepository) CreateTask(ctx context.Context, task *listingkit.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*listingkit.Task, error) {
	var task listingkit.Task
	if err := r.db.WithContext(ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, listingkit.ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) ListTasks(ctx context.Context, query *listingkit.TaskListQuery) ([]listingkit.Task, int64, error) {
	page, pageSize := normalizeTaskListPage(query)
	db := r.db.WithContext(ctx).Model(&listingkit.Task{})
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
	return r.db.WithContext(ctx).Model(&listingkit.Task{}).Where("id = ?", taskID).UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).Error
}

func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *listingkit.ListingKitResult) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result})
}

func (r *taskRepository) MutateTaskResult(ctx context.Context, taskID string, mutate listingkit.TaskResultMutation) (*listingkit.Task, error) {
	var out *listingkit.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task listingkit.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", taskID).First(&task).Error; err != nil {
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

func (r *taskRepository) GetCanonicalProductCache(ctx context.Context, fingerprint string) (*productenrich.CanonicalProduct, error) {
	if fingerprint == "" {
		return nil, nil
	}
	var entry listingkit.CanonicalProductCacheEntry
	if err := r.db.WithContext(ctx).Where("fingerprint = ?", fingerprint).First(&entry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return entry.CanonicalProduct()
}

func (r *taskRepository) SaveCanonicalProductCache(ctx context.Context, fingerprint string, product *productenrich.CanonicalProduct, sourceTaskID string) error {
	entry, err := listingkit.NewCanonicalProductCacheEntry(fingerprint, product, sourceTaskID)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "fingerprint"}},
			DoUpdates: clause.Assignments(map[string]any{
				"product":        entry.Product,
				"source_task_id": sourceTaskID,
				"updated_at":     gorm.Expr("NOW()"),
			}),
		}).
		Create(entry).Error
}

func (r *taskRepository) updateTaskFields(ctx context.Context, taskID string, updates map[string]any) error {
	updates["updated_at"] = gorm.Expr("NOW()")
	result := r.db.WithContext(ctx).Model(&listingkit.Task{}).Where("id = ?", taskID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return listingkit.ErrTaskNotFound
	}
	return nil
}
