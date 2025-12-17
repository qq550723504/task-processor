// Package adapters 提供SHEIN平台适配器实现
package adapters

import (
	"context"
	"fmt"

	"task-processor/internal/common/processor"
	"task-processor/internal/common/types"
	"task-processor/internal/dispatcher"
	"task-processor/internal/model"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/shein/modules"

	"github.com/sirupsen/logrus"
)

// SheinProcessorAdapter SHEIN平台处理器适配器
type SheinProcessorAdapter struct {
	*processor.BaseProcessorAdapter
	processor *shein.SheinProcessor
}

// NewSheinProcessorAdapter 创建SHEIN处理器适配器
func NewSheinProcessorAdapter(sheinProcessor *shein.SheinProcessor, logger *logrus.Logger) dispatcher.PlatformProcessor {
	return &SheinProcessorAdapter{
		BaseProcessorAdapter: processor.NewBaseProcessorAdapter("SHEIN", logger),
		processor:            sheinProcessor,
	}
}

// ProcessTask 处理任务
func (s *SheinProcessorAdapter) ProcessTask(ctx context.Context, task *model.UnifiedTask) error {
	// 执行基础任务处理逻辑
	if err := s.ProcessTaskBase(task); err != nil {
		return err
	}

	// 转换为SHEIN Task
	sheinTask, err := s.convertToSheinTask(task)
	if err != nil {
		s.OnTaskFailure(task.ID, fmt.Errorf("转换SHEIN任务失败: %w", err))
		return fmt.Errorf("转换SHEIN任务失败: %w", err)
	}

	// 调用SHEIN处理器
	typesTask := (*types.Task)(sheinTask)
	err = s.processor.ProcessTask(ctx, typesTask)
	if err != nil {
		s.OnTaskFailure(task.ID, fmt.Errorf("SHEIN处理器执行失败: %w", err))
		return fmt.Errorf("SHEIN处理器执行失败: %w", err)
	}

	// 处理成功
	s.OnTaskSuccess(task.ID)
	return nil
}

// convertToSheinTask 将UnifiedTask转换为SHEIN Task
func (s *SheinProcessorAdapter) convertToSheinTask(task *model.UnifiedTask) (*modules.Task, error) {
	// 创建SHEIN Task
	sheinTask := &modules.Task{
		ID:         task.ID,
		TenantID:   task.TenantID,
		ProductID:  task.ProductID,
		Platform:   task.Platform,
		Region:     task.Region,
		StoreID:    task.StoreID,
		CategoryID: task.CategoryID,
		CreateTime: task.CreateTime,
		RetryCount: task.RetryCount,
		Priority:   task.Priority,
		Creator:    task.Creator,
	}

	// 设置默认值
	if sheinTask.Platform == "" {
		sheinTask.Platform = task.SourcePlatform
	}
	if sheinTask.Region == "" {
		sheinTask.Region = "US" // 默认美国地区
	}

	return sheinTask, nil
}

// Start 启动处理器
func (s *SheinProcessorAdapter) Start(ctx context.Context) error {
	// 执行基础启动逻辑
	s.StartBase(ctx)

	// 启动底层SHEIN处理器
	if err := s.processor.Start(ctx); err != nil {
		return fmt.Errorf("启动SHEIN处理器失败: %w", err)
	}

	return nil
}

// Stop 停止处理器
func (s *SheinProcessorAdapter) Stop(ctx context.Context) error {
	// 执行基础停止逻辑
	s.StopBase(ctx)

	// 停止底层SHEIN处理器
	s.processor.Close()

	return nil
}

// GetStatus 获取处理器状态
func (s *SheinProcessorAdapter) GetStatus() *dispatcher.ProcessorStatus {
	// 获取工作池状态并更新可用槽位
	if workerPool := s.processor.GetWorkerPool(); workerPool != nil {
		s.UpdateAvailableSlots(workerPool.AvailableSlots())
	}

	// 返回基础状态
	return s.GetStatusBase()
}

// CanHandle 检查是否可以处理指定任务
func (s *SheinProcessorAdapter) CanHandle(task *model.UnifiedTask) bool {
	return s.CanHandleBase(task, "shein")
}

// GetProcessor 获取底层SHEIN处理器（用于测试或特殊需求）
func (s *SheinProcessorAdapter) GetProcessor() *shein.SheinProcessor {
	return s.processor
}

// GetMemoryManager 获取内存管理器
func (s *SheinProcessorAdapter) GetMemoryManager() interface{} {
	if s.processor != nil {
		return s.processor.GetMemoryManager()
	}
	return nil
}

// GetShopClientManager 获取店铺客户端管理器
func (s *SheinProcessorAdapter) GetShopClientManager() interface{} {
	if s.processor != nil {
		return s.processor.GetShopClientManager()
	}
	return nil
}

// GetManagementClientManager 获取管理客户端管理器
func (s *SheinProcessorAdapter) GetManagementClientManager() interface{} {
	if s.processor != nil {
		return s.processor.GetManagementClient()
	}
	return nil
}
