// Package productenrich provides product enrichment functionality.
package productenrich

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// taskRepository 是 TaskRepository 的实现
type taskRepository struct {
	db *gorm.DB
}

// NewTaskRepository 创建新的任务仓库实例
func NewTaskRepository(db *gorm.DB) TaskRepository {
	return &taskRepository{
		db: db,
	}
}

// CreateTask 创建新任务
func (r *taskRepository) CreateTask(ctx context.Context, task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	result := r.db.WithContext(ctx).Create(task)
	if result.Error != nil {
		return fmt.Errorf("failed to create task: %w", result.Error)
	}

	return nil
}

// GetTask 根据 ID 获取任务
func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*Task, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	var task Task
	result := r.db.WithContext(ctx).Where("id = ?", taskID).First(&task)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", result.Error)
	}

	return &task, nil
}

// UpdateTaskStatus 更新任务状态
func (r *taskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status TaskStatus) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	result := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ?", taskID).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("failed to update task status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// SaveTaskResult 保存任务结果
func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *ProductJSON) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	updates := map[string]any{
		"result": result,
		"status": TaskStatusCompleted,
	}

	dbResult := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ?", taskID).
		Updates(updates)

	if dbResult.Error != nil {
		return fmt.Errorf("failed to save task result: %w", dbResult.Error)
	}

	if dbResult.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}

// IncrementRetryCount 增加重试次数
func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID string) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	result := r.db.WithContext(ctx).
		Model(&Task{}).
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

// UpdateTaskError 更新任务错误信息
func (r *taskRepository) UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	updates := map[string]any{
		"error":  errorMsg,
		"status": TaskStatusFailed,
	}

	result := r.db.WithContext(ctx).
		Model(&Task{}).
		Where("id = ?", taskID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update task error: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("task not found: %s", taskID)
	}

	return nil
}
