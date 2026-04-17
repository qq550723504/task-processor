package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
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
