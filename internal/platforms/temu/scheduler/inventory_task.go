// Package scheduler 提供TEMU平台库存同步任务实现
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu/api/client"
	temuscheduler "task-processor/internal/platforms/temu/service/scheduler"

	"github.com/sirupsen/logrus"
)

// InventoryTask TEMU库存同步任务
type InventoryTask struct {
	*BaseTask
	managementClient *management.ClientManager
	temuAPIClient    client.APIClientInterface
	inventoryService temuscheduler.InventorySyncService
	logger           *logrus.Entry
}

// NewInventoryTask 创建库存同步任务
func NewInventoryTask(
	ctx context.Context,
	config appscheduler.TaskConfig,
	managementClient *management.ClientManager,
	temuAPIClient client.APIClientInterface,
	amazonProcessor *amazon.AmazonProcessor,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
) *InventoryTask {
	baseTask := NewBaseTask(config)

	// 创建库存同步服务
	rawJsonDataClient := managementClient.GetRawJsonDataClient()
	inventoryRecordClient := managementClient.GetInventoryRecordClient()

	inventoryService := temuscheduler.NewInventorySyncService(
		managementClient,
		temuAPIClient,
		amazonProcessor,
		amazonConfig,
		monitorConfig,
		rawJsonDataClient,
		inventoryRecordClient,
	)

	return &InventoryTask{
		BaseTask:         baseTask,
		managementClient: managementClient,
		temuAPIClient:    temuAPIClient,
		inventoryService: inventoryService,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuInventoryTask",
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TenantID,
			"store_id":  config.StoreID,
		}),
	}
}

// Execute 执行库存同步任务
func (t *InventoryTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行TEMU库存同步任务")

	// 1. 获取需要监控库存的产品列表
	products, err := t.inventoryService.FetchProductsForInventorySync(ctx, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("获取TEMU产品列表失败: %w", err)
	}

	if len(products) == 0 {
		t.logger.Info("没有需要监控的TEMU产品")
		return nil
	}

	// 2. 监控库存和价格变化（包含Amazon数据获取、对比、更新Attributes、记录历史）
	result, err := t.inventoryService.MonitorInventoryChanges(ctx, products, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("监控TEMU库存变化失败: %w", err)
	}

	// 3. 输出任务执行结果
	t.logger.WithFields(logrus.Fields{
		"total_products":     result.TotalProducts,
		"processed_products": result.ProcessedProducts,
		"skipped_products":   result.SkippedProducts,
		"price_changes":      result.PriceChanges,
		"stock_changes":      result.StockChanges,
		"amazon_fetched":     result.AmazonFetched,
		"amazon_failed":      result.AmazonFailed,
	}).Info("TEMU库存同步任务执行完成")

	return nil
}
