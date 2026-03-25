// package scheduler 提供SHEIN平台库存同步任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	platformtask "task-processor/internal/platformtask"
)

// InventoryTask SHEIN库存同步任务
type InventoryTask struct {
	*platformtask.InventorySyncTask
}

// NewInventoryTask 创建库存同步任务
func NewInventoryTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	inventoryService platformtask.InventorySyncService,
) *InventoryTask {
	_ = ctx

	baseTask := platformtask.NewInventorySyncTask(platformtask.InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		InventoryService: inventoryService,
		PlatformName:     "SHEIN",
	})

	return &InventoryTask{
		InventorySyncTask: baseTask,
	}
}

// Execute 执行库存同步任务
func (t *InventoryTask) Execute(ctx context.Context) error {
	return t.InventorySyncTask.Execute(ctx)
}
