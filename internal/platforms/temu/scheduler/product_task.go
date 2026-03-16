// Package scheduler 提供TEMU平台同步任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"
	commonscheduler "task-processor/internal/platforms/taskbase"
	temuscheduler "task-processor/internal/platforms/temu/syncsvc"
)

// ProductSyncTask TEMU产品同步任务
// 使用通用基类实现
type ProductSyncTask struct {
	*commonscheduler.ProductSyncTask
}

// NewProductSyncTask 创建TEMU产品同步任务
func NewProductSyncTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	syncService temuscheduler.ProductSyncService,
) *ProductSyncTask {
	// 创建适配器，将TEMU特定服务适配到通用接口
	adapter := newProductSyncServiceAdapter(syncService)

	// 使用通用基类创建任务
	baseTask := commonscheduler.NewProductSyncTask(commonscheduler.ProductSyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		SyncService:      adapter,
		PlatformName:     "TEMU",
	})

	return &ProductSyncTask{
		ProductSyncTask: baseTask,
	}
}

// Execute 执行TEMU产品同步任务
// 直接使用基类的Execute方法
func (t *ProductSyncTask) Execute(ctx context.Context) error {
	return t.ProductSyncTask.Execute(ctx)
}
