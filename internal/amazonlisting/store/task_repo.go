package store

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
)

type taskRepository struct {
	db *gorm.DB
}

func NewTaskRepository(db *gorm.DB) amazonlisting.Repository {
	return &taskRepository{db: db}
}

func (r *taskRepository) CreateTask(ctx context.Context, task *amazonlisting.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*amazonlisting.Task, error) {
	var task amazonlisting.Task
	if err := r.db.WithContext(ctx).Where("id = ?", taskID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, amazonlisting.ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) MarkProcessing(ctx context.Context, taskID string) error {
	return r.UpdateTaskStatus(ctx, taskID, amazonlisting.TaskStatusProcessing)
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID string, result *amazonlisting.AmazonListingDraft) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result, "status": amazonlisting.TaskStatusCompleted, "error": ""})
}

func (r *taskRepository) MarkNeedsReview(ctx context.Context, taskID string, result *amazonlisting.AmazonListingDraft, reason string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result, "status": amazonlisting.TaskStatusNeedsReview, "error": reason})
}

func (r *taskRepository) MarkRejected(ctx context.Context, taskID string, reason string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"status": amazonlisting.TaskStatusRejected, "error": reason})
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"status": amazonlisting.TaskStatusFailed, "error": errorMsg})
}

func (r *taskRepository) PrepareRetry(ctx context.Context, taskID string) error {
	return r.ResetForRetry(ctx, taskID)
}

func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	return r.db.WithContext(ctx).Model(&amazonlisting.Task{}).Where("id = ?", taskID).UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).Error
}

func (r *taskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status amazonlisting.TaskStatus) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"status": status})
}

func (r *taskRepository) UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"error": errorMsg})
}

func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *amazonlisting.AmazonListingDraft) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"result": result})
}

func (r *taskRepository) ResetForRetry(ctx context.Context, taskID string) error {
	return r.updateTaskFields(ctx, taskID, map[string]any{"status": amazonlisting.TaskStatusPending, "error": ""})
}

func (r *taskRepository) updateTaskFields(ctx context.Context, taskID string, updates map[string]any) error {
	updates["updated_at"] = gorm.Expr("NOW()")
	result := r.db.WithContext(ctx).Model(&amazonlisting.Task{}).Where("id = ?", taskID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return amazonlisting.ErrTaskNotFound
	}
	return nil
}
