// package scheduler 提供SHEIN平台核价任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	platformtask "task-processor/internal/platformtask"
	"task-processor/internal/shein/client"
	sheinpricing "task-processor/internal/shein/pricing"
)

// PricingTask SHEIN核价任务
// 已废弃：使用通用的AutoPricingTask替代
// 保留此类型以保持向后兼容
type PricingTask struct {
	*platformtask.AutoPricingTask
	adapter *SheinAutoPricingAdapter
}

// NewPricingTask 创建核价任务
func NewPricingTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	clientManager *client.ClientManager,
	pricingService sheinpricing.AutoPricingService,
) *PricingTask {
	// 创建适配器
	adapter := NewSheinAutoPricingAdapter(pricingService)

	// 创建通用自动核价任务
	autoPricingTask := platformtask.NewAutoPricingTask(platformtask.AutoPricingTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		PricingService:   adapter,
		PlatformName:     "Shein",
	})

	return &PricingTask{
		AutoPricingTask: autoPricingTask,
		adapter:         adapter,
	}
}

// Execute 执行核价任务
// 使用通用基类的Execute方法
func (t *PricingTask) Execute(ctx context.Context) error {
	return t.AutoPricingTask.Execute(ctx)
}
