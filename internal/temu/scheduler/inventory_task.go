// Package scheduler 提供TEMU平台库存同步任务实现
package scheduler

import (
"context"

appscheduler "task-processor/internal/app/scheduler"
"task-processor/internal/infra/clients/management"
platformtask "task-processor/internal/platformtask"
"task-processor/internal/temu/api/client"
temuscheduler "task-processor/internal/temu/sync"
)

// InventoryTask TEMU库存同步任务
type InventoryTask struct {
*platformtask.InventorySyncTask
temuAPIClient client.ClientAPI
}

// NewInventoryTask 创建库存同步任务
func NewInventoryTask(
ctx context.Context,
config appscheduler.TaskConfig,
managementClient *management.ClientManager,
inventoryService temuscheduler.InventorySyncService,
) *InventoryTask {
adapter := newInventorySyncServiceAdapter(inventoryService)

baseTask := platformtask.NewInventorySyncTask(platformtask.InventorySyncTaskConfig{
TaskConfig:       config,
ManagementClient: managementClient,
InventoryService: adapter,
PlatformName:     "TEMU",
})

return &InventoryTask{
InventorySyncTask: baseTask,
}
}

// Execute 执行库存同步任务
func (t *InventoryTask) Execute(ctx context.Context) error {
return t.InventorySyncTask.Execute(ctx)
}