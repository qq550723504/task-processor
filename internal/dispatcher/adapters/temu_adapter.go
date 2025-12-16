// Package adapters 提供TEMU平台适配器实现
package adapters

import (
	"context"
	"fmt"

	"task-processor/common/processor"
	"task-processor/common/types"
	"task-processor/internal/dispatcher"
	"task-processor/internal/model"
	"task-processor/platforms/temu"

	"github.com/sirupsen/logrus"
)

// TemuProcessorAdapter TEMU平台处理器适配器
type TemuProcessorAdapter struct {
	*processor.BaseProcessorAdapter
	processor *temu.TemuProcessor
}

// NewTemuProcessorAdapter 创建TEMU处理器适配器
func NewTemuProcessorAdapter(temuProcessor *temu.TemuProcessor, logger *logrus.Logger) dispatcher.PlatformProcessor {
	return &TemuProcessorAdapter{
		BaseProcessorAdapter: processor.NewBaseProcessorAdapter("TEMU", logger),
		processor:            temuProcessor,
	}
}

// ProcessTask 处理任务
func (t *TemuProcessorAdapter) ProcessTask(ctx context.Context, task *model.UnifiedTask) error {
	// 执行基础任务处理逻辑
	if err := t.ProcessTaskBase(task); err != nil {
		return err
	}

	// 转换为TEMU Task
	temuTask, err := t.convertToTemuTask(task)
	if err != nil {
		t.OnTaskFailure(task.ID, fmt.Errorf("转换TEMU任务失败: %w", err))
		return fmt.Errorf("转换TEMU任务失败: %w", err)
	}

	// 调用TEMU处理器
	err = t.processor.ProcessTask(ctx, *temuTask)
	if err != nil {
		t.OnTaskFailure(task.ID, fmt.Errorf("TEMU处理器执行失败: %w", err))
		return fmt.Errorf("TEMU处理器执行失败: %w", err)
	}

	// 处理成功
	t.OnTaskSuccess(task.ID)
	return nil
}

// convertToTemuTask 将UnifiedTask转换为TEMU Task
func (t *TemuProcessorAdapter) convertToTemuTask(task *model.UnifiedTask) (*types.Task, error) {
	// 创建TEMU Task
	temuTask := &types.Task{
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
	if temuTask.Platform == "" {
		temuTask.Platform = task.SourcePlatform
	}
	if temuTask.Region == "" {
		temuTask.Region = "US" // 默认美国地区
	}

	return temuTask, nil
}

// Start 启动处理器
func (t *TemuProcessorAdapter) Start(ctx context.Context) error {
	// 执行基础启动逻辑
	t.StartBase(ctx)

	// 启动底层TEMU处理器
	if err := t.processor.Start(ctx); err != nil {
		return fmt.Errorf("启动TEMU处理器失败: %w", err)
	}

	return nil
}

// Stop 停止处理器
func (t *TemuProcessorAdapter) Stop(ctx context.Context) error {
	// 执行基础停止逻辑
	t.StopBase(ctx)

	// 停止底层TEMU处理器
	t.processor.Close()

	return nil
}

// GetStatus 获取处理器状态
func (t *TemuProcessorAdapter) GetStatus() *dispatcher.ProcessorStatus {
	// 获取工作池状态并更新可用槽位
	if workerPool := t.processor.GetWorkerPool(); workerPool != nil {
		t.UpdateAvailableSlots(workerPool.AvailableSlots())
	}

	// 返回基础状态
	return t.GetStatusBase()
}

// CanHandle 检查是否可以处理指定任务
func (t *TemuProcessorAdapter) CanHandle(task *model.UnifiedTask) bool {
	return t.CanHandleBase(task, "temu")
}

// GetProcessor 获取底层TEMU处理器（用于测试或特殊需求）
func (t *TemuProcessorAdapter) GetProcessor() *temu.TemuProcessor {
	return t.processor
}

// SetUserToken 设置用户访问令牌
func (t *TemuProcessorAdapter) SetUserToken(accessToken, tenantID string) {
	if t.processor != nil {
		t.processor.SetUserToken(accessToken, tenantID)
	}
}

// GetManagementClient 获取管理系统客户端
func (t *TemuProcessorAdapter) GetManagementClient() interface{} {
	if t.processor != nil {
		return t.processor.GetManagementClient()
	}
	return nil
}
