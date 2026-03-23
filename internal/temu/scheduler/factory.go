// Package scheduler 提供TEMU平台的任务工厂
package scheduler

import (
"context"
"fmt"

appscheduler "task-processor/internal/app/scheduler"
"task-processor/internal/app/crawler/fetcher"
"task-processor/internal/core/config"
"task-processor/internal/core/logger"
"task-processor/internal/infra/clients/management"
"task-processor/internal/infra/rabbitmq"
"task-processor/internal/platformbase"
temuapi "task-processor/internal/temu/api"
"task-processor/internal/temu/api/client"
schedulerservice "task-processor/internal/temu/sync"
)

// TemuTaskFactory TEMU平台任务工厂
type TemuTaskFactory struct {
*platformbase.BaseFactory
clientManager  *client.APIClientManager
rabbitmqClient *rabbitmq.Client
}

// NewTemuTaskFactory 创建TEMU任务工厂
func NewTemuTaskFactory(
managementClient *management.ClientManager,
amazonProcessor platformbase.AmazonCrawler,
amazonConfig *config.AmazonConfig,
monitorConfig *config.MonitorConfig,
rabbitmqClient *rabbitmq.Client,
) *TemuTaskFactory {
clientManager := client.NewAPIClientManager(managementClient)

baseFactory := platformbase.NewBaseFactory(platformbase.BaseFactoryConfig{
Platform:         "TEMU",
ManagementClient: managementClient,
AmazonProcessor:  amazonProcessor,
AmazonConfig:     amazonConfig,
MonitorConfig:    monitorConfig,
})

return &TemuTaskFactory{
BaseFactory:    baseFactory,
clientManager:  clientManager,
rabbitmqClient: rabbitmqClient,
}
}

// CreateTask 创建任务
func (f *TemuTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	// 使用基类验证平台和任务类型
	if err := f.ValidatePlatform(config); err != nil {
		return nil, err
	}
	if err := f.ValidateTaskType(config.TaskType); err != nil {
		return nil, err
	}

	switch config.TaskType {
	case appscheduler.TaskTypePricing:
		return NewPricingTask(ctx, config, f.GetManagementClient()), nil
	case appscheduler.TaskTypeProductSync:
		return f.createProductSyncTask(ctx, config)
	case appscheduler.TaskTypeInventory:
		return f.createInventoryTask(ctx, config)
	case appscheduler.TaskTypeActivity:
		return NewActivityTask(ctx, config, f.GetManagementClient()), nil
	default:
		return nil, fmt.Errorf("不支持的任务类型: %s", config.TaskType)
	}
}

// createProductSyncTask 创建产品同步任务
func (f *TemuTaskFactory) createProductSyncTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	// 获取 API 客户端
	apiClient, err := f.clientManager.GetClient(config.TenantID, config.StoreID)
	if err != nil {
		return nil, fmt.Errorf("获取TEMU API客户端失败: %w", err)
	}

	// 创建 ProductAPI
	productAPI := temuapi.NewProductAPI(apiClient, logger.GetGlobalLogger("TemuProductAPI"))

	// 创建 SkuQueryAPI
	skuQueryAPI := temuapi.NewQueryAPI(apiClient, logger.GetGlobalLogger("TemuSkuQueryAPI"))

	// 获取映射客户端
	mappingClient := f.GetManagementClient().GetProductImportMappingClient()

	// 获取店铺客户端
	storeAPI := f.GetManagementClient().GetStoreClient()

	// 创建产品同步服务配置
	syncConfig := &schedulerservice.ProductSyncConfig{
		PageSize:        100,
		MaxPages:        0, // 暂时只处理一页数据用于调试
		Language:        "en",
		IncludeInactive: false,
	}

	// 创建产品同步服务
	syncService := schedulerservice.NewProductSyncService(
		f.GetManagementClient(),
		productAPI,
		skuQueryAPI,
		mappingClient,
		storeAPI,
		syncConfig,
	)

	return NewProductSyncTask(ctx, config, f.GetManagementClient(), syncService), nil
}

// createInventoryTask 创建库存同步任务
func (f *TemuTaskFactory) createInventoryTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
temuAPIClient, err := f.clientManager.GetClient(config.TenantID, config.StoreID)
if err != nil {
return nil, fmt.Errorf("获取TEMU API客户端失败: %w", err)
}

rawJsonDataClient := f.GetManagementClient().GetRawJsonDataAdapter()
inventoryRecordClient := f.GetManagementClient().GetInventoryRecordClient()

fetcherType := fetcher.LocalFetcher
if f.rabbitmqClient != nil {
fetcherType = fetcher.DistributedFetcher
}
productFetcher, err := fetcher.NewFetcherFactory().CreateFetcher(
fetcherType,
rawJsonDataClient,
f.GetAmazonConfig(),
f.GetAmazonProcessor(),
f.rabbitmqClient,
)
if err != nil {
return nil, fmt.Errorf("创建产品获取器失败: %w", err)
}

inventoryService := schedulerservice.NewInventorySyncService(
f.GetManagementClient(),
temuAPIClient,
productFetcher,
f.GetMonitorConfig(),
rawJsonDataClient,
inventoryRecordClient,
)

return NewInventoryTask(ctx, config, f.GetManagementClient(), inventoryService), nil
}

// SupportedTaskTypes 支持的任务类型
func (f *TemuTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	// TEMU平台支持所有基础任务类型
	return f.BaseFactory.SupportedTaskTypes()
}
