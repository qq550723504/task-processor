// Package platformtask 提供平台通用的调度任务基础实现
package platformtask

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/infra/clients/management"

	"github.com/sirupsen/logrus"
)

// InventorySyncResult 库存同步结果
type InventorySyncResult struct {
	TotalProducts     int // 总产品数
	ProcessedProducts int // 已处理产品数
	SkippedProducts   int // 跳过的产品数
	PriceChanges      int // 价格变化数
	StockChanges      int // 库存变化数
	AmazonFetched     int // Amazon数据获取成功数
	AmazonFailed      int // Amazon数据获取失败数
}

// InventorySyncService 库存同步服务接口
// 各平台需要实现此接口
type InventorySyncService interface {
	// FetchProductsForInventorySync 获取需要监控库存的产品列表
	FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]any, error)

	// MonitorInventoryChanges 监控库存和价格变化
	MonitorInventoryChanges(ctx context.Context, products []any, tenantID, storeID int64) (*InventorySyncResult, error)
}

// InventorySyncTask 通用库存同步任务
type InventorySyncTask struct {
	*BaseTask
	managementClient *management.ClientManager
	inventoryService InventorySyncService
	logger           *logrus.Entry
	platformName     string
}

// InventorySyncTaskConfig 库存同步任务配置
type InventorySyncTaskConfig struct {
	TaskConfig       appscheduler.TaskConfig
	ManagementClient *management.ClientManager
	InventoryService InventorySyncService
	PlatformName     string
}

// NewInventorySyncTask 创建通用库存同步任务
func NewInventorySyncTask(config InventorySyncTaskConfig) *InventorySyncTask {
	baseTask := NewBaseTask(config.TaskConfig)

	return &InventorySyncTask{
		BaseTask:         baseTask,
		managementClient: config.ManagementClient,
		inventoryService: config.InventoryService,
		platformName:     config.PlatformName,
		logger: logrus.WithFields(logrus.Fields{
			"component": fmt.Sprintf("%sInventoryTask", config.PlatformName),
			"task_id":   baseTask.GetID(),
			"tenant_id": config.TaskConfig.TenantID,
			"store_id":  config.TaskConfig.StoreID,
		}),
	}
}

// Execute 执行库存同步任务
func (t *InventorySyncTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Infof("开始执行%s库存同步任务", t.platformName)

	// 1. 获取需要监控库存的产品列表
	products, err := t.inventoryService.FetchProductsForInventorySync(ctx, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("获取%s产品列表失败: %w", t.platformName, err)
	}

	t.logger.Infof("需要监控库存的%s产品数量: %d", t.platformName, len(products))

	if len(products) == 0 {
		t.logger.Infof("没有需要监控库存的%s产品", t.platformName)
		return nil
	}

	// 2. 监控库存和价格变化
	result, err := t.inventoryService.MonitorInventoryChanges(ctx, products, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("监控%s库存变化失败: %w", t.platformName, err)
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
	}).Infof("%s库存同步任务执行完成", t.platformName)

	return nil
}

// GetManagementClient 获取管理客户端
func (t *InventorySyncTask) GetManagementClient() *management.ClientManager {
	return t.managementClient
}

// GetInventoryService 获取库存同步服务
func (t *InventorySyncTask) GetInventoryService() InventorySyncService {
	return t.inventoryService
}
