// Package scheduler 提供TEMU平台同步任务实现
package scheduler

import (
	"context"

	platformtask "task-processor/internal/platformtask"
	appscheduler "task-processor/internal/scheduler"
)

// ProductSyncTask TEMU产品同步任务
type ProductSyncTask struct {
	*platformtask.ProductSyncTask
}

// NewProductSyncTask 创建TEMU产品同步任务
func NewProductSyncTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	syncService platformtask.ProductSyncService,
) *ProductSyncTask {
	_ = ctx

	baseTask := platformtask.NewProductSyncTask(platformtask.ProductSyncTaskConfig{
		TaskConfig:   config,
		SyncService:  syncService,
		PlatformName: "TEMU",
	})

	return &ProductSyncTask{
		ProductSyncTask: baseTask,
	}
}

// Execute 执行TEMU产品同步任务
func (t *ProductSyncTask) Execute(ctx context.Context) error {
	return t.ProductSyncTask.Execute(ctx)
}
