// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/core/logger"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// CreateGenerateTask 创建产品生成任务
func (s *productService) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if err := s.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// 生成唯一的任务 ID
	taskID := s.generateTaskID()

	// 创建任务
	task := &Task{
		ID:         taskID,
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}

	// 保存任务到数据库
	if err := s.taskRepo.CreateTask(ctx, task); err != nil {
		logger.GetGlobalLogger("productenrich/service_task.go").WithFields(logrus.Fields{
			"task_id": taskID,
		}).WithError(err).Error("failed to create task in database")
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 将任务提交到 Worker Pool
	if s.taskSubmitter != nil && shouldEnqueueTask(ctx) {
		if err := s.taskSubmitter.Submit(taskID); err != nil {
			logger.GetGlobalLogger("productenrich/service_task.go").WithField("task_id", taskID).WithError(err).Error("failed to submit task to worker pool")
			// Submit 失败时将任务标记为 failed，避免留下永久 pending 的孤儿任务
			if dbErr := s.taskRepo.UpdateTaskError(ctx, taskID, fmt.Sprintf("failed to submit task: %v", err)); dbErr != nil {
				logger.GetGlobalLogger("productenrich/service_task.go").WithField("task_id", taskID).WithError(dbErr).Error("failed to mark orphan task as failed")
			}
			return nil, fmt.Errorf("failed to submit task: %w", err)
		}
	} else if s.redisClient != nil && shouldEnqueueTask(ctx) {
		// 降级：无 Pool 时写入 Redis 队列（兼容旧模式）
		if err := s.redisClient.Push(ctx, s.queueName, taskID); err != nil {
			logger.GetGlobalLogger("productenrich/service_task.go").WithField("task_id", taskID).WithError(err).Error("failed to push task to queue")
			if dbErr := s.taskRepo.UpdateTaskError(ctx, taskID, fmt.Sprintf("failed to enqueue task: %v", err)); dbErr != nil {
				logger.GetGlobalLogger("productenrich/service_task.go").WithField("task_id", taskID).WithError(dbErr).Error("failed to mark orphan task as failed")
			}
			return nil, fmt.Errorf("failed to enqueue task: %w", err)
		}
	} else if shouldEnqueueTask(ctx) {
		logger.GetGlobalLogger("productenrich/service_task.go").WithField("task_id", taskID).Warn("no worker pool or redis configured, task will not be processed automatically")
	}

	logger.GetGlobalLogger("productenrich/service_task.go").WithFields(logrus.Fields{
		"task_id": taskID,
		"status":  string(task.Status),
	}).Info("task created successfully")

	return task, nil
}

// GetTaskResult 获取任务结果
func (s *productService) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	// 从数据库获取任务
	task, err := s.taskRepo.GetTask(ctx, taskID)
	if err != nil {
		logger.GetGlobalLogger("productenrich/service_task.go").WithFields(logrus.Fields{
			"task_id": taskID,
		}).WithError(err).Error("failed to get task")
		return nil, err
	}

	// 构建任务结果
	result := &TaskResult{
		TaskID:      task.ID,
		Status:      task.Status,
		ProductJSON: task.Result,
		Error:       task.Error,
		CreatedAt:   task.CreatedAt,
	}

	// 如果任务已完成，设置完成时间
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		result.CompletedAt = &task.UpdatedAt
	}

	return result, nil
}

// validateRequest 验证请求
func (s *productService) validateRequest(req *GenerateRequest) error {
	// 至少需要提供一种输入
	if len(req.ImageURLs) == 0 && req.Text == "" && req.ProductURL == "" {
		return fmt.Errorf("at least one input type is required (image_urls, text, or product_url)")
	}

	// 验证图片 URL
	if len(req.ImageURLs) > 10 {
		return fmt.Errorf("too many image URLs (max 10)")
	}

	// 验证文本长度
	if len(req.Text) > 10000 {
		return fmt.Errorf("text too long (max 10000 characters)")
	}

	// product_url 当前仅支持 1688 商品页，避免异步阶段才失败
	if req.ProductURL != "" && !isSupportedProductURL(req.ProductURL) {
		return fmt.Errorf("product_url must be a valid 1688 product page URL")
	}

	return nil
}

func isSupportedProductURL(rawURL string) bool {
	return is1688ProductDetailURL(rawURL)
}

// generateTaskID 生成唯一的任务 ID
func (s *productService) generateTaskID() string {
	return uuid.New().String()
}
