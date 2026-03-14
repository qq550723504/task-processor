// Package scheduler 提供SHEIN平台的任务工厂
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/app/state"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/platforms/common/factory"
	"task-processor/internal/platforms/shein/repo"
	"task-processor/internal/platforms/shein/repo/client"
	schedulerservice "task-processor/internal/platforms/shein/service/business_service"
)

// SheinTaskFactory SHEIN平台任务工厂
type SheinTaskFactory struct {
	*factory.BaseFactoryImpl
	cookieManager *state.CookieManager
	clientManager *client.ClientManager
}

// NewSheinTaskFactory 创建SHEIN任务工厂
func NewSheinTaskFactory(managementClient *management.ClientManager, amazonProcessor *amazon.AmazonProcessor, amazonConfig *config.AmazonConfig, monitorConfig *config.MonitorConfig) *SheinTaskFactory {
	cookieManager := state.NewCookieManager()
	clientManager := client.NewClientManager(cookieManager, managementClient)

	baseFactory := factory.NewBaseFactory(factory.BaseFactoryConfig{
		Platform:         "SHEIN",
		ManagementClient: managementClient,
		AmazonProcessor:  amazonProcessor,
		AmazonConfig:     amazonConfig,
		MonitorConfig:    monitorConfig,
	})

	return &SheinTaskFactory{
		BaseFactoryImpl: baseFactory,
		cookieManager:   cookieManager,
		clientManager:   clientManager,
	}
}

// CreateTask 创建任务
func (f *SheinTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	// 使用基类验证平台和任务类型
	if err := f.ValidatePlatform(config); err != nil {
		return nil, err
	}
	if err := f.ValidateTaskType(config.TaskType); err != nil {
		return nil, err
	}

	// 获取店铺信息和 BaseAPIClient（公共逻辑）
	baseClient, err := f.createBaseClient(config.StoreID)
	if err != nil {
		return nil, err
	}

	switch config.TaskType {
	case appscheduler.TaskTypePricing:
		return f.createPricingTask(ctx, config, baseClient)
	case appscheduler.TaskTypeProductSync:
		return f.createProductSyncTask(ctx, config, baseClient)
	case appscheduler.TaskTypeInventory:
		return f.createInventoryTask(ctx, config, baseClient)
	case appscheduler.TaskTypeActivity:
		return f.createActivityTask(ctx, config, baseClient)
	default:
		return nil, fmt.Errorf("不支持的任务类型: %s", config.TaskType)
	}
}

// createBaseClient 创建基础API客户端
func (f *SheinTaskFactory) createBaseClient(storeID int64) (*client.BaseAPIClient, error) {
	// 获取店铺信息
	storeInfo, err := f.GetManagementClient().GetStoreClient().GetStore(storeID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 获取 API 客户端
	apiClient, err := f.clientManager.GetClient(storeID, storeInfo)
	if err != nil {
		return nil, fmt.Errorf("获取API客户端失败: %w", err)
	}

	// 创建 BaseAPIClient
	baseClient := client.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		apiClient.GetStoreID(),
		apiClient.GetHTTPClient(),
	)

	return baseClient, nil
}

// createPricingTask 创建核价任务
func (f *SheinTaskFactory) createPricingTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	pricingAPI := repo.NewPricingAPI(baseClient)
	pricingService := schedulerservice.NewAutoPricingService(f.GetManagementClient(), pricingAPI)
	return NewPricingTask(ctx, config, f.GetManagementClient(), f.clientManager, pricingService), nil
}

// createProductSyncTask 创建产品同步任务
func (f *SheinTaskFactory) createProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	// 创建错误处理器
	errorHandler := client.NewAPIErrorHandler(baseClient)

	// 创建所需的 API 和管理器
	productAPI := repo.NewProductAPI(baseClient)
	inventoryManager := repo.NewInventoryManager(baseClient, errorHandler)
	priceManager := repo.NewPriceManager(baseClient, errorHandler)
	storeInfoClient := f.GetManagementClient().GetStoreClient()
	// 获取映射客户端
	mappingClient := f.GetManagementClient().GetProductImportMappingClient()

	// 创建产品同步服务
	syncService := schedulerservice.NewProductSyncService(
		f.GetManagementClient(),
		productAPI,
		inventoryManager,
		priceManager,
		mappingClient,
		storeInfoClient,
	)

	return NewProductSyncTask(ctx, config, f.GetManagementClient(), f.clientManager, syncService), nil
}

// createInventoryTask 创建库存监控任务
func (f *SheinTaskFactory) createInventoryTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	// 创建 ProductAPI
	productAPI := repo.NewProductAPI(baseClient)

	// 获取 RawJsonData 客户端
	rawJsonDataClient := f.GetManagementClient().GetRawJsonDataClient()

	// 获取 InventoryRecord 客户端
	inventoryRecordClient := f.GetManagementClient().GetInventoryRecordClient()

	// 创建库存监控服务
	inventoryService := schedulerservice.NewInventorySyncService(
		f.GetManagementClient(),
		productAPI,
		f.GetAmazonProcessor(),
		f.GetAmazonConfig(),
		f.GetMonitorConfig(),
		rawJsonDataClient,
		inventoryRecordClient,
	)

	return NewInventoryTask(ctx, config, f.GetManagementClient(), f.clientManager, inventoryService), nil
}

// createActivityTask 创建活动报名任务
func (f *SheinTaskFactory) createActivityTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	marketingAPI := repo.NewMarketingAPI(baseClient)
	activityService := schedulerservice.NewActivityRegistrationService(f.GetManagementClient(), marketingAPI)
	return NewActivityTask(ctx, config, f.GetManagementClient(), f.clientManager, activityService), nil
}

// SupportedTaskTypes 支持的任务类型
func (f *SheinTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	// SHEIN平台支持所有基础任务类型
	return f.BaseFactoryImpl.SupportedTaskTypes()
}
