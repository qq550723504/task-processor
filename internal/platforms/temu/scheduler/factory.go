// Package scheduler 提供TEMU平台的任务工厂
package scheduler

import (
	"context"
	"fmt"

	appscheduler "task-processor/internal/app/scheduler"
	"task-processor/internal/pkg/management"
	"task-processor/internal/platforms/temu/api/client"
	"task-processor/internal/platforms/temu/api/services"
	schedulerservice "task-processor/internal/platforms/temu/service/scheduler"

	"github.com/sirupsen/logrus"
)

// TemuTaskFactory TEMU平台任务工厂
type TemuTaskFactory struct {
	managementClient *management.ClientManager
	clientManager    *client.APIClientManager
	logger           *logrus.Entry
}

// NewTemuTaskFactory 创建TEMU任务工厂
func NewTemuTaskFactory(
	managementClient *management.ClientManager,
) *TemuTaskFactory {
	clientManager := client.NewAPIClientManager(managementClient)

	return &TemuTaskFactory{
		managementClient: managementClient,
		clientManager:    clientManager,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TemuTaskFactory",
		}),
	}
}

// CreateTask 创建任务
func (f *TemuTaskFactory) CreateTask(ctx context.Context, config appscheduler.TaskConfig) (appscheduler.Task, error) {
	if config.Platform != "TEMU" {
		return nil, fmt.Errorf("不支持的平台: %s", config.Platform)
	}

	switch config.TaskType {
	case appscheduler.TaskTypePricing:
		return NewPricingTask(ctx, config, f.managementClient), nil
	case appscheduler.TaskTypeProductSync:
		return f.createProductSyncTask(ctx, config)
	case appscheduler.TaskTypeInventory:
		return NewInventoryTask(ctx, config, f.managementClient), nil
	case appscheduler.TaskTypeActivity:
		return NewActivityTask(ctx, config, f.managementClient), nil
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
	productAPI := services.NewProductAPI(apiClient, logrus.WithField("component", "TemuProductAPI"))

	// 创建 SkuQueryAPI
	skuQueryAPI := services.NewSkuQueryAPI(apiClient, logrus.WithField("component", "TemuSkuQueryAPI"))

	// 获取映射客户端
	mappingClient := f.managementClient.GetProductImportMappingClient()

	// 获取店铺客户端
	storeAPI := f.managementClient.GetStoreClient()

	// 创建产品同步服务配置
	syncConfig := &schedulerservice.ProductSyncConfig{
		PageSize:        100,
		MaxPages:        1, // 暂时只处理一页数据用于调试
		Language:        "en",
		IncludeInactive: false,
	}

	// 创建产品同步服务
	syncService := schedulerservice.NewProductSyncService(
		f.managementClient,
		productAPI,
		skuQueryAPI,
		mappingClient,
		storeAPI,
		syncConfig,
	)

	return NewProductSyncTask(ctx, config, f.managementClient, syncService), nil
}

// SupportedPlatform 支持的平台
func (f *TemuTaskFactory) SupportedPlatform() string {
	return "TEMU"
}

// SupportedTaskTypes 支持的任务类型
func (f *TemuTaskFactory) SupportedTaskTypes() []appscheduler.TaskType {
	return []appscheduler.TaskType{
		appscheduler.TaskTypePricing,
		appscheduler.TaskTypeProductSync,
		appscheduler.TaskTypeInventory,
		appscheduler.TaskTypeActivity,
	}
}
