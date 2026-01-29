// Package scheduler 提供TEMU平台同步任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/repo/client"
	schedulerservice "task-processor/internal/platforms/temu/service/scheduler"

	"github.com/sirupsen/logrus"
)

// ProductSyncTask TEMU产品同步任务
type ProductSyncTask struct {
	*BaseTask
	managementClient *management.ClientManager
	clientManager    *client.ClientManager
	syncService      schedulerservice.ProductSyncService
	logger           *logrus.Entry
}

// NewProductSyncTask 创建TEMU产品同步任务
func NewProductSyncTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	syncService schedulerservice.ProductSyncService,
) *ProductSyncTask {
	baseTask := NewBaseTask(config)

	return &ProductSyncTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		syncService:      syncService,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuProductSyncTask",
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TenantID,
			"store_id":  config.StoreID,
		}),
	}
}

// Execute 执行TEMU产品同步任务
func (t *ProductSyncTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行TEMU产品同步任务")

	// 1. 从TEMU API获取产品列表
	products, err := t.syncService.FetchProductList(ctx)
	if err != nil {
		return fmt.Errorf("获取TEMU产品列表失败: %w", err)
	}

	t.logger.Infof("获取到 %d 个TEMU产品", len(products))

	// 2. 转换为后端格式
	productDataList, err := t.syncService.ConvertProducts(ctx, products, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("转换TEMU产品格式失败: %w", err)
	}

	// 3. 批量保存到管理系统
	savedCount, err := t.syncService.SaveProducts(ctx, productDataList)
	if err != nil {
		return fmt.Errorf("保存TEMU产品失败: %w", err)
	}

	// 4. 记录同步统计
	t.logger.Infof("TEMU产品同步任务执行完成，成功同步 %d 个产品", savedCount)
	return nil
}
