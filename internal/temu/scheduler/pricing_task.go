// Package scheduler 提供TEMU平台核价任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/temu/api"
)

// PricingTask TEMU核价任务
// 已废弃：使用通用的AutoPricingTask替代
// 保留此类型以保持向后兼容
type PricingTask struct {
	*platformtask.AutoPricingTask
	adapter *TemuAutoPricingAdapter
}

// NewPricingTask 创建核价任务
func NewPricingTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
) *PricingTask {
	// 创建API客户端
	apiClient := api.NewAPIClient(config.StoreID, managementClient)
	if apiClient == nil {
		// 如果创建失败，返回nil
		// 调用方需要检查返回值
		return nil
	}

	// 创建适配器
	adapter := NewTemuAutoPricingAdapter(apiClient, managementClient)

	// 创建通用自动核价任务
	autoPricingTask := platformtask.NewAutoPricingTask(platformtask.AutoPricingTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		PricingService:   adapter,
		PlatformName:     "Temu",
	})

	return &PricingTask{
		AutoPricingTask: autoPricingTask,
		adapter:         adapter,
	}
}

// Execute 执行核价任务
// 重写Execute方法以处理Temu特有的逻辑
func (t *PricingTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	// 检查任务是否正确初始化
	if t.AutoPricingTask == nil {
		return fmt.Errorf("任务未正确初始化")
	}

	// 调用通用基类的Execute方法
	// 注意：Temu的实现会在SubmitPricingResults中完成所有工作
	return t.AutoPricingTask.Execute(ctx)
}
