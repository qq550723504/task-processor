// package scheduler 提供SHEIN平台库存监控任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/shein/client"
	"task-processor/internal/shein/inventory"
	commonscheduler "task-processor/internal/taskbase"
)

// InventoryTask SHEIN库存监控任务
// 使用通用基类实现
type InventoryTask struct {
	*commonscheduler.InventorySyncTask
	clientManager *client.ClientManager // 保留SHEIN特定的字段
}

// NewInventoryTask 创建库存同步任务
func NewInventoryTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	clientManager *client.ClientManager,
	inventoryService inventory.InventorySyncService,
) *InventoryTask {
	// 创建适配器
	adapter := newInventorySyncServiceAdapter(inventoryService)

	// 使用通用基类创建任务
	baseTask := commonscheduler.NewInventorySyncTask(commonscheduler.InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		InventoryService: adapter,
		PlatformName:     "SHEIN",
	})

	return &InventoryTask{
		InventorySyncTask: baseTask,
		clientManager:     clientManager,
	}
}

// Execute 执行库存监控任务
// 直接使用基类的Execute方法
func (t *InventoryTask) Execute(ctx context.Context) error {
	return t.InventorySyncTask.Execute(ctx)
}
