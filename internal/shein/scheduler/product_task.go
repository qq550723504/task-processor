// package scheduler 提供SHEIN平台同步任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	platformtask "task-processor/internal/platformtask"
)

// ProductSyncTask SHEIN产品同步任务
type ProductSyncTask struct {
	*platformtask.ProductSyncTask
}

// NewProductSyncTask 创建SHEIN产品同步任务
func NewProductSyncTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	syncService platformtask.ProductSyncService,
) *ProductSyncTask {
	_ = ctx

	baseTask := platformtask.NewProductSyncTask(platformtask.ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		SyncService:      syncService,
		PlatformName:     "SHEIN",
	})

	return &ProductSyncTask{
		ProductSyncTask: baseTask,
	}
}

// Execute 执行SHEIN产品同步任务
func (t *ProductSyncTask) Execute(ctx context.Context) error {
	return t.ProductSyncTask.Execute(ctx)
}
