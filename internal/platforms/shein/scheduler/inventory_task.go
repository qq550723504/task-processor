// Package scheduler 提供SHEIN平台库存监控任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/repo/client"
	schedulerservice "task-processor/internal/platforms/shein/service/scheduler"

	"github.com/sirupsen/logrus"
)

// InventoryTask SHEIN库存监控任务
type InventoryTask struct {
	*BaseTask
	managementClient *management.ClientManager
	clientManager    *client.ClientManager
	inventoryService schedulerservice.InventorySyncService
	logger           *logrus.Entry
}

// NewInventoryTask 创建库存同步任务
func NewInventoryTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	clientManager *client.ClientManager,
	inventoryService schedulerservice.InventorySyncService,
) *InventoryTask {
	baseTask := NewBaseTask(config)

	return &InventoryTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		clientManager:    clientManager,
		inventoryService: inventoryService,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SheinInventoryTask",
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TenantID,
			"store_id":  config.StoreID,
		}),
	}
}

// Execute 执行库存监控任务
func (t *InventoryTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行SHEIN库存监控任务")

	// 1. 获取需要监控库存的产品列表
	products, err := t.inventoryService.FetchProductsForInventorySync(ctx, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("获取产品列表失败: %w", err)
	}

	t.logger.Infof("需要监控库存的产品数量: %d", len(products))

	if len(products) == 0 {
		t.logger.Info("没有需要监控库存的产品")
		return nil
	}

	// 2. 监控库存和价格变化（包含Amazon数据获取、对比、更新Attributes、记录历史）
	result, err := t.inventoryService.MonitorInventoryChanges(ctx, products, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("监控库存变化失败: %w", err)
	}

	// 3. 记录监控结果
	t.logger.WithFields(logrus.Fields{
		"total_products":     result.TotalProducts,
		"processed_products": result.ProcessedProducts,
		"skipped_products":   result.SkippedProducts,
		"price_changes":      result.PriceChanges,
		"stock_changes":      result.StockChanges,
		"amazon_fetched":     result.AmazonFetched,
		"amazon_failed":      result.AmazonFailed,
	}).Info("SHEIN库存监控任务执行完成")

	return nil
}
