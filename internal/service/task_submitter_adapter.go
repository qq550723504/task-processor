// Package service 提供任务提交器适配器
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"task-processor/internal/common/processor"
	"task-processor/internal/common/task"
	"task-processor/internal/common/types"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"

	"github.com/sirupsen/logrus"
)

// TemuTaskSubmitter TEMU任务提交器适配器
type TemuTaskSubmitter struct {
	processor *temu.TemuProcessor
	logger    *logrus.Logger
}

// NewTemuTaskSubmitter 创建TEMU任务提交器
func NewTemuTaskSubmitter(processor *temu.TemuProcessor, logger *logrus.Logger) task.TaskSubmitter {
	return &TemuTaskSubmitter{
		processor: processor,
		logger:    logger,
	}
}

// SubmitTask 提交任务到TEMU处理器
func (t *TemuTaskSubmitter) SubmitTask(taskData string) error {
	// 解析任务数据
	var task types.Task
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return fmt.Errorf("解析TEMU任务数据失败: %w", err)
	}

	t.logger.Infof("[TEMU提交器] 提交任务: ID=%s, ProductID=%s", task.ID, task.ProductID)

	// 提交给处理器
	ctx := context.Background()
	if err := t.processor.ProcessTask(ctx, &task); err != nil {
		return fmt.Errorf("TEMU处理器处理任务失败: %w", err)
	}

	t.logger.Infof("[TEMU提交器] 任务提交成功: ID=%s", task.ID)
	return nil
}

// GetPlatform 获取平台类型
func (t *TemuTaskSubmitter) GetPlatform() string {
	return "temu"
}

// GetAvailableSlots 获取可用槽位数
func (t *TemuTaskSubmitter) GetAvailableSlots() int {
	// TODO: 从处理器获取实际的可用槽位数
	return 10 // 暂时返回固定值
}

// GetQueueStats 获取队列统计信息
func (t *TemuTaskSubmitter) GetQueueStats() processor.QueueStats {
	// TODO: 从处理器获取实际的队列统计
	return processor.QueueStats{
		QueueSize:      0,
		BufferSize:     10,
		AvailableSlots: 10,
		UsagePercent:   0.0,
	}
}

// SheinTaskSubmitter SHEIN任务提交器适配器
type SheinTaskSubmitter struct {
	processor *shein.SheinProcessor
	logger    *logrus.Logger
}

// NewSheinTaskSubmitter 创建SHEIN任务提交器
func NewSheinTaskSubmitter(processor *shein.SheinProcessor, logger *logrus.Logger) task.TaskSubmitter {
	return &SheinTaskSubmitter{
		processor: processor,
		logger:    logger,
	}
}

// SubmitTask 提交任务到SHEIN处理器
func (s *SheinTaskSubmitter) SubmitTask(taskData string) error {
	// 解析任务数据
	var task types.Task
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return fmt.Errorf("解析SHEIN任务数据失败: %w", err)
	}

	s.logger.Infof("[SHEIN提交器] 提交任务: ID=%s, ProductID=%s", task.ID, task.ProductID)

	// 提交给处理器
	ctx := context.Background()
	if err := s.processor.ProcessTask(ctx, &task); err != nil {
		return fmt.Errorf("SHEIN处理器处理任务失败: %w", err)
	}

	s.logger.Infof("[SHEIN提交器] 任务提交成功: ID=%s", task.ID)
	return nil
}

// GetPlatform 获取平台类型
func (s *SheinTaskSubmitter) GetPlatform() string {
	return "shein"
}

// GetAvailableSlots 获取可用槽位数
func (s *SheinTaskSubmitter) GetAvailableSlots() int {
	// TODO: 从处理器获取实际的可用槽位数
	return 10 // 暂时返回固定值
}

// GetQueueStats 获取队列统计信息
func (s *SheinTaskSubmitter) GetQueueStats() processor.QueueStats {
	// TODO: 从处理器获取实际的队列统计
	return processor.QueueStats{
		QueueSize:      0,
		BufferSize:     10,
		AvailableSlots: 10,
		UsagePercent:   0.0,
	}
}
