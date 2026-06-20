// package scheduler 提供SHEIN平台核价任务实现
package scheduler

import (
	"context"

	platformtask "task-processor/internal/platformtask"
	appscheduler "task-processor/internal/scheduler"
)

// PricingTask SHEIN核价任务
// 已废弃：使用通用的 AutoPricingTask 替代
// 保留该类型以保持向后兼容
type PricingTask struct {
	*platformtask.AutoPricingTask
}

// NewPricingTask 创建核价任务
func NewPricingTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	storeConfigProvider platformtask.AutoPricingStoreConfigProvider,
	pricingService platformtask.AutoPricingService,
) *PricingTask {
	_ = ctx

	autoPricingTask := platformtask.NewAutoPricingTask(platformtask.AutoPricingTaskConfig{
		TaskConfig:          config,
		StoreConfigProvider: storeConfigProvider,
		PricingService:      pricingService,
		PlatformName:        "Shein",
	})

	return &PricingTask{
		AutoPricingTask: autoPricingTask,
	}
}

// Execute 执行核价任务
func (t *PricingTask) Execute(ctx context.Context) error {
	return t.AutoPricingTask.Execute(ctx)
}
