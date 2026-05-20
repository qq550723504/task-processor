// Package scheduler 提供TEMU平台核价任务实现
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/infra/clients/management"
	platformtask "task-processor/internal/platformtask"
	appscheduler "task-processor/internal/scheduler"
)

// PricingTask TEMU核价任务
// 已废弃：使用通用的 AutoPricingTask 替代
// 保留该类型以保持向后兼容
type PricingTask struct {
	*platformtask.AutoPricingTask
}

// NewPricingTask 创建核价任务
func NewPricingTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	pricingService platformtask.AutoPricingService,
) *PricingTask {
	_ = ctx

	autoPricingTask := platformtask.NewAutoPricingTask(platformtask.AutoPricingTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		PricingService:   pricingService,
		PlatformName:     "Temu",
	})

	return &PricingTask{
		AutoPricingTask: autoPricingTask,
	}
}

// Execute 执行核价任务
func (t *PricingTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	if t.AutoPricingTask == nil {
		return fmt.Errorf("任务未正确初始化")
	}

	return t.AutoPricingTask.Execute(ctx)
}
