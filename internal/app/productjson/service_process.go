// Package productjson 提供产品JSON生成的应用层实现
package productjson

import (
	"context"
	"fmt"

	domain "task-processor/internal/domain/productjson"

	"github.com/sirupsen/logrus"
)

// ProcessProduct 处理产品生成（由 Worker 调用）
func (s *productService) ProcessProduct(ctx context.Context, task *domain.Task) (*domain.ProductJSON, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	logrus.WithField("task_id", task.ID).Info("starting product processing")

	// 更新任务状态为处理中
	if err := s.taskRepo.UpdateTaskStatus(ctx, task.ID, domain.TaskStatusProcessing); err != nil {
		logrus.WithField("task_id", task.ID).WithError(err).Error("failed to update task status to processing")
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// 步骤 1: 解析输入
	parsedInput, err := s.parseInput(ctx, task)
	if err != nil {
		s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("input parsing failed: %v", err))
		return nil, err
	}

	// 步骤 2: 验证输入并选择策略
	_, err = s.validateAndSelectStrategy(ctx, task, parsedInput)
	if err != nil {
		return nil, err
	}

	// 步骤 3: 分析产品
	analysis, err := s.analyzeProduct(ctx, task, parsedInput)
	if err != nil {
		s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("product analysis failed: %v", err))
		return nil, err
	}

	// 步骤 4: 生成 JSON
	productJSON, err := s.generateProductJSON(ctx, task, analysis)
	if err != nil {
		s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("JSON generation failed: %v", err))
		return nil, err
	}

	// 添加原始图片 URL
	if len(task.Request.ImageURLs) > 0 {
		productJSON.Images = task.Request.ImageURLs
	}

	// 步骤 5: 验证生成结果
	s.validateResult(ctx, task, parsedInput, productJSON)

	// 保存任务结果
	logrus.WithField("task_id", task.ID).Info("saving task result")
	if err := s.taskRepo.SaveTaskResult(ctx, task.ID, productJSON); err != nil {
		logrus.WithField("task_id", task.ID).WithError(err).Error("failed to save task result")
		s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("failed to save result: %v", err))
		return nil, fmt.Errorf("failed to save task result: %w", err)
	}

	logrus.WithField("task_id", task.ID).Info("task completed successfully")
	return productJSON, nil
}
