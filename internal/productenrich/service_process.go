// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"

	"task-processor/internal/core/logger"
)

// ProcessProduct 处理产品生成（由 Worker 调用）
func (s *productService) ProcessProduct(ctx context.Context, task *Task) (*ProductJSON, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}

	logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).Info("starting product processing")

	// 更新任务状态为处理中
	if err := s.taskRepo.UpdateTaskStatus(ctx, task.ID, TaskStatusProcessing); err != nil {
		logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(err).Error("failed to update task status to processing")
		return nil, fmt.Errorf("failed to update task status: %w", err)
	}

	// 步骤 1: 解析输入
	parsedInput, err := s.parseInput(ctx, task)
	if err != nil {
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("input parsing failed: %v", err)); dbErr != nil {
			logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return nil, err
	}

	// 步骤 2: 验证输入并选择策略
	strategy, err := s.validateAndSelectStrategy(ctx, task, parsedInput)
	if err != nil {
		return nil, err
	}

	// 步骤 3: 分析产品
	analysis, err := s.analyzeProduct(ctx, task, parsedInput)
	if err != nil {
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("product analysis failed: %v", err)); dbErr != nil {
			logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return nil, err
	}

	// 步骤 4: 生成 JSON（minimal 策略跳过变体生成）
	productJSON, err := s.generateProductJSON(ctx, task, analysis, strategy)
	if err != nil {
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("JSON generation failed: %v", err)); dbErr != nil {
			logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return nil, err
	}

	// 用 parsedInput.Images 作为最终图片列表（已包含原始图片和 scraped 图片，且已去重）
	// LLM 不生成图片 URL，直接覆盖
	productJSON.Images = parsedInput.Images

	// 步骤 5: 验证生成结果
	if err := s.validateResult(ctx, task, parsedInput, productJSON); err != nil {
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, err.Error()); dbErr != nil {
			logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return nil, err
	}

	// 保存任务结果
	logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).Info("saving task result")
	if err := s.taskRepo.SaveTaskResult(ctx, task.ID, productJSON); err != nil {
		logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(err).Error("failed to save task result")
		if dbErr := s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("failed to save result: %v", err)); dbErr != nil {
			logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).WithError(dbErr).Error("failed to persist task error")
		}
		return nil, fmt.Errorf("failed to save task result: %w", err)
	}

	logger.GetGlobalLogger("productenrich/service_process.go").WithField("task_id", task.ID).Info("task completed successfully")
	return productJSON, nil
}
