// Package taskexecutor 提供SHEIN平台的任务工厂
package taskexecutor

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/app/state"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/platforms/platformbase"
	"task-processor/internal/platforms/shein/api/marketing"
	shein_pricing "task-processor/internal/platforms/shein/api/pricing"
	shein_product "task-processor/internal/platforms/shein/api/product"
	"task-processor/internal/platforms/shein/client"
	schedulerservice "task-processor/internal/platforms/shein/operation"
)

// SheinTaskFactory SHEIN平台任务工厂
type SheinTaskFactory struct {
	*platformbase.BaseFactory
	cookieManager *state.CookieManager
	clientManager *client.ClientManager
}

// NewSheinTaskFactory 创建SHEIN任务工厂
func NewSheinTaskFactory(managementClient *management.ClientManager, amazonProcessor *amazon.AmazonProcessor, amazonConfig *config.AmazonConfig, monitorConfig *config.MonitorConfig) *SheinTaskFactory {
	cookieManager := state.NewCookieManager()
	clientManager := client.NewClientManager(cookieManager, managementClient)

	baseFactory := platformbase.NewBaseFactory(platformbase.BaseFactoryConfig{
		Platform:         "SHEIN",
		ManagementClient: managementClient,
		AmazonProcessor:  amazonProcessor,
		AmazonConfig:     amazonConfig,
		MonitorConfig:    monitorConfig,
	})

	return &SheinTaskFactory{
		BaseFactory:   baseFactory,
		cookieManager: cookieManager,
		clientManager: clientManager,
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
	pricingAPI := shein_pricing.NewClient(baseClient)
	pricingService := schedulerservice.NewAutoPricingService(f.GetManagementClient(), pricingAPI)
	return NewPricingTask(ctx, config, f.GetManagementClient(), f.clientManager, pricingService), nil
}

// createProductSyncTask 创建产品同步任务
func (f *SheinTaskFactory) createProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	errorHandler := client.NewAPIErrorHandler(baseClient)

	productAPI := shein_product.NewClient(baseClient)
	inventoryManager := shein_product.NewInventoryManager(baseClient, errorHandler)
	priceManager := shein_product.NewPriceManager(baseClient, errorHandler)
	storeInfoClient := f.GetManagementClient().GetStoreClient()
	mappingClient := f.GetManagementClient().GetProductImportMappingClient()

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
	productAPI := shein_product.NewClient(baseClient)
	rawJsonDataClient := f.GetManagementClient().GetRawJsonDataAdapter()
	inventoryRecordClient := f.GetManagementClient().GetInventoryRecordClient()

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
	marketingAPI := marketing.NewClient(baseClient)
	activityService := schedulerservice.NewActivityRegistrationService(f.GetManagementClient(), marketingAPI)
	return NewActivityTask(ctx, config, f.GetManagementClient(), f.clientManager, activityService), nil
}

// SupportedTaskTypes 支持的任务类型
func (f *SheinTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	// SHEIN平台支持所有基础任务类型
	return f.BaseFactory.SupportedTaskTypes()
}
