// Package database 提供数据访问层实现
package database

import (
	"context"
	"errors"
	"fmt"

	"task-processor/internal/domain/productjson"

	"gorm.io/gorm"
)

// TaskRepository 定义任务仓库接口
type TaskRepository interface {
	// CreateTask 创建新任务
	CreateTask(ctx context.Context, task *productjson.Task) error
	// GetTask 根据 ID 获取任务
	GetTask(ctx context.Context, taskID string) (*productjson.Task, error)
	// UpdateTaskStatus 更新任务状态
	UpdateTaskStatus(ctx context.Context, taskID string, status productjson.TaskStatus) error
	// SaveTaskResult 保存任务结果
	SaveTaskResult(ctx context.Context, taskID string, result *productjson.ProductJSON) error
	// IncrementRetryCount 增加重试次数
	IncrementRetryCount(ctx context.Context, taskID string) error
	// UpdateTaskError 更新任务错误信息
	UpdateTaskError(ctx context.Context, taskID string, errorMsg string) error
}

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
func (r *taskRepository) CreateTask(ctx context.Context, task *productjson.Task) error {
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
func (r *taskRepository) GetTask(ctx context.Context, taskID string) (*productjson.Task, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	var task productjson.Task
	result := r.db.WithContext(ctx).Where("id = ?", taskID).First(&task)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to get task: %w", result.Error)
	}

	return &task, nil
}

// UpdateTaskStatus 更新任务状态
func (r *taskRepository) UpdateTaskStatus(ctx context.Context, taskID string, status productjson.TaskStatus) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}

	result := r.db.WithContext(ctx).
		Model(&productjson.Task{}).
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
func (r *taskRepository) SaveTaskResult(ctx context.Context, taskID string, result *productjson.ProductJSON) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	// 更新任务结果和状态
	updates := map[string]any{
		"result": result,
		"status": productjson.TaskStatusCompleted,
	}

	dbResult := r.db.WithContext(ctx).
		Model(&productjson.Task{}).
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
		Model(&productjson.Task{}).
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
		"status": productjson.TaskStatusFailed,
	}

	result := r.db.WithContext(ctx).
		Model(&productjson.Task{}).
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
