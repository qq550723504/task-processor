package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	productimage "task-processor/internal/productimage"
)

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) productimage.TaskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) CreateTask(ctx context.Context, task *productimage.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	result := r.db.WithContext(ctx).Create(task)
	if result.Error != nil {
		return fmt.Errorf("failed to create task: %w", result.Error)
	}
	return nil
}

func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*productimage.Task, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}
	var task productimage.Task
	result := r.db.WithContext(ctx).Where("id = ?", taskID).First(&task)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, productimage.ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", result.Error)
	}
	return &task, nil
}

func (r *taskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": productimage.TaskStatusProcessing,
		"error":  "",
	})
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID string, result *productimage.ImageProcessResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
		"status": productimage.TaskStatusCompleted,
		"error":  "",
	})
}

func (r *taskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *productimage.ImageProcessResult, reason string) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
		"status": productimage.TaskStatusNeedsReview,
		"error":  reason,
	})
}

func (r *taskRepository) MarkRejected(ctx context.Context, taskID string, reason string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": productimage.TaskStatusRejected,
		"error":  reason,
	})
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.UpdateTaskError(ctx, taskID, errorMsg)
}

func (r *taskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	return r.ResetForRetry(ctx, taskID)
}

func (r *taskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status productimage.TaskStatus) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": status,
	})
}

func (r *taskRepository) UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": productimage.TaskStatusFailed,
		"error":  errorMsg,
	})
}

func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *productimage.ImageProcessResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"result": result,
	})
}

func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	result := r.db.WithContext(ctx).
		Model(&productimage.Task{}).
		Where("id = ?", taskID).
		UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1))
	if result.Error != nil {
		return fmt.Errorf("failed to increment retry count: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}
	return nil
}

func (r *taskRepository) ResetForRetry(ctx context.Context, taskID string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{
		"status": productimage.TaskStatusPending,
		"error":  "",
	})
}

func (r *taskRepository) updateTaskFields(ctx context.Context, taskID string, updates map[string]any) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	updates["updated_at"] = gorm.Expr("NOW()")
	result := r.db.WithContext(ctx).
		Model(&productimage.Task{}).
		Where("id = ?", taskID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}
	return nil
}
