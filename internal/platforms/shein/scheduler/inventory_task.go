// Package scheduler 提供SHEIN平台库存同步任务实现
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

// InventoryTask SHEIN库存同步任务
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

// Execute 执行库存同步任务
func (t *InventoryTask) Execute(ctx context.Context) error {
	t.SetStatus(appscheduler.TaskStatusRunning)
	defer t.SetStatus(appscheduler.TaskStatusStopped)

	t.logger.Info("开始执行SHEIN库存同步任务")

	// 1. 获取需要同步库存的产品列表
	products, err := t.inventoryService.FetchProductsForInventorySync(ctx, t.GetTenantID(), t.GetStoreID())
	if err != nil {
		return fmt.Errorf("获取产品列表失败: %w", err)
	}

	t.logger.Infof("需要同步库存的产品数量: %d", len(products))

	if len(products) == 0 {
		t.logger.Info("没有需要同步库存的产品")
		return nil
	}

	// 2. 从SHEIN API获取最新库存信息
	inventoryMap, err := t.inventoryService.FetchInventoryFromShein(ctx, products)
	if err != nil {
		return fmt.Errorf("获取库存信息失败: %w", err)
	}

	// 3. 更新到管理系统
	updatedCount, err := t.inventoryService.UpdateInventoryToManagement(ctx, products, inventoryMap)
	if err != nil {
		return fmt.Errorf("更新库存失败: %w", err)
	}

	// 4. 记录同步结果
	t.logger.Infof("SHEIN库存同步任务执行完成，成功更新 %d 个产品的库存", updatedCount)
	return nil
}
