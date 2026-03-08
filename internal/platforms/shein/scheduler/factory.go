// Package scheduler 提供SHEIN平台的任务工厂
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/state"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/shein/repo"
	"task-processor/internal/platforms/shein/repo/client"
	schedulerservice "task-processor/internal/platforms/shein/service/scheduler"

	"github.com/sirupsen/logrus"
)

// SheinTaskFactory SHEIN平台任务工厂
type SheinTaskFactory struct {
	managementClient *management.ClientManager
	cookieManager    *state.CookieManager
	clientManager    *client.ClientManager
	amazonProcessor  *amazon.AmazonProcessor
	amazonConfig     *config.AmazonConfig
	monitorConfig    *config.MonitorConfig
	logger           *logrus.Entry
}

// NewSheinTaskFactory 创建SHEIN任务工厂
func NewSheinTaskFactory(managementClient *management.ClientManager, amazonProcessor *amazon.AmazonProcessor, amazonConfig *config.AmazonConfig, monitorConfig *config.MonitorConfig) *SheinTaskFactory {
	cookieManager := state.NewCookieManager()
	clientManager := client.NewClientManager(cookieManager, managementClient)

	return &SheinTaskFactory{
		managementClient: managementClient,
		cookieManager:    cookieManager,
		clientManager:    clientManager,
		amazonProcessor:  amazonProcessor,
		amazonConfig:     amazonConfig,
		monitorConfig:    monitorConfig,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SheinTaskFactory",
		}),
	}
}

// CreateTask 创建任务
func (f *SheinTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	if config.Platform != "SHEIN" {
		return nil, fmt.Errorf("不支持的平台: %s", config.Platform)
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
	storeInfo, err := f.managementClient.GetStoreClient().GetStore(storeID)
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
	pricingService := schedulerservice.NewAutoPricingService(f.managementClient, pricingAPI)
	return NewPricingTask(ctx, config, f.managementClient, f.clientManager, pricingService), nil
}

// createProductSyncTask 创建产品同步任务
func (f *SheinTaskFactory) createProductSyncTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	// 创建错误处理器
	errorHandler := client.NewAPIErrorHandler(baseClient)

	// 创建所需的 API 和管理器
	productAPI := repo.NewProductAPI(baseClient)
	inventoryManager := repo.NewInventoryManager(baseClient, errorHandler)
	priceManager := repo.NewPriceManager(baseClient, errorHandler)
	storeInfoClient := f.managementClient.GetStoreClient()
	// 获取映射客户端
	mappingClient := f.managementClient.GetProductImportMappingClient()

	// 创建产品同步服务
	syncService := schedulerservice.NewProductSyncService(
		f.managementClient,
		productAPI,
		inventoryManager,
		priceManager,
		mappingClient,
		storeInfoClient,
	)

	return NewProductSyncTask(ctx, config, f.managementClient, f.clientManager, syncService), nil
}

// createInventoryTask 创建库存监控任务
func (f *SheinTaskFactory) createInventoryTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	// 创建 ProductAPI
	productAPI := repo.NewProductAPI(baseClient)

	// 获取 RawJsonData 客户端
	rawJsonDataClient := f.managementClient.GetRawJsonDataClient()

	// 获取 InventoryRecord 客户端
	inventoryRecordClient := f.managementClient.GetInventoryRecordClient()

	// 创建库存监控服务
	inventoryService := schedulerservice.NewInventorySyncService(
		f.managementClient,
		productAPI,
		f.amazonProcessor,
		f.amazonConfig,
		f.monitorConfig,
		rawJsonDataClient,
		inventoryRecordClient,
	)

	return NewInventoryTask(ctx, config, f.managementClient, f.clientManager, inventoryService), nil
}

// createActivityTask 创建活动报名任务
func (f *SheinTaskFactory) createActivityTask(ctx context.Context, config appscheduler.TaskConfig, baseClient *client.BaseAPIClient) (appscheduler.Task, error) {
	marketingAPI := repo.NewMarketingAPI(baseClient)
	activityService := schedulerservice.NewActivityRegistrationService(f.managementClient, marketingAPI)
	return NewActivityTask(ctx, config, f.managementClient, f.clientManager, activityService), nil
}

// SupportedPlatform 支持的平台
func (f *SheinTaskFactory) SupportedPlatform() string {
	return "SHEIN"
}

// SupportedTaskTypes 支持的任务类型
func (f *SheinTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}
}
