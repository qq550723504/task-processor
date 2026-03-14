// Package scheduler 提供TEMU平台库存同步任务实现
package scheduler

import (
	"context"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	commonscheduler "task-processor/internal/platforms/common/scheduler"
	"task-processor/internal/platforms/temu/api/client"
	temuscheduler "task-processor/internal/platforms/temu/services/business_service"
)

// InventoryTask TEMU库存同步任务
// 使用通用基类实现
type InventoryTask struct {
	*commonscheduler.InventorySyncTask
	temuAPIClient client.APIClientInterface // 保留TEMU特定的字段
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

	// 创建适配器
	adapter := newInventorySyncServiceAdapter(inventoryService)

	// 使用通用基类创建任务
	baseTask := commonscheduler.NewInventorySyncTask(commonscheduler.InventorySyncTaskConfig{
		TaskConfig:       config,
		ManagementClient: managementClient,
		InventoryService: adapter,
		PlatformName:     "TEMU",
	})

	return &InventoryTask{
		InventorySyncTask: baseTask,
		temuAPIClient:     temuAPIClient,
	}
}

// Execute 执行库存同步任务
// 直接使用基类的Execute方法
func (t *InventoryTask) Execute(ctx context.Context) error {
	return t.InventorySyncTask.Execute(ctx)
}
