// package scheduler 提供SHEIN平台同步任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/shein/client"
	"task-processor/internal/shein/productsync"
	platformtask "task-processor/internal/platformtask"
)

// ProductSyncTask SHEIN产品同步任务
// 使用通用基类实现
type ProductSyncTask struct {
	*platformtask.ProductSyncTask
	clientManager *client.ClientManager // 保留SHEIN特定的字段
}

// NewProductSyncTask 创建SHEIN产品同步任务
func NewProductSyncTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	clientManager *client.ClientManager,
	syncService productsync.ProductSyncService,
) *ProductSyncTask {
	// 创建适配器，将SHEIN特定服务适配到通用接口
	adapter := newProductSyncServiceAdapter(syncService)

	// 使用通用基类创建任务
	baseTask := platformtask.NewProductSyncTask(platformtask.ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		SyncService:      adapter,
		PlatformName:     "SHEIN",
	})

	return &ProductSyncTask{
		ProductSyncTask: baseTask,
		clientManager:   clientManager,
	}
}

// Execute 执行SHEIN产品同步任务
// 直接使用基类的Execute方法
func (t *ProductSyncTask) Execute(ctx context.Context) error {
	return t.ProductSyncTask.Execute(ctx)
}


