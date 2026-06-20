// package scheduler 提供SHEIN平台库存同步任务实现
package scheduler

import (
	"context"

	platformtask "task-processor/internal/platformtask"
	appscheduler "task-processor/internal/scheduler"
)

// InventoryTask SHEIN库存同步任务
type InventoryTask struct {
	*platformtask.InventorySyncTask
}

// NewInventoryTask 创建库存同步任务
func NewInventoryTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	inventoryService platformtask.InventorySyncService,
) *InventoryTask {
	_ = ctx

	baseTask := platformtask.NewInventorySyncTask(platformtask.InventorySyncTaskConfig{
		TaskConfig:       config,
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
