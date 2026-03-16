// Package syncsvc 提供TEMU库存监控服务工厂
package syncsvc

import (
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/platforms/temu/api/client"

	"github.com/sirupsen/logrus"
)

// InventorySyncServiceFactory TEMU库存监控服务工厂
type InventorySyncServiceFactory struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewInventorySyncServiceFactory 创建库存监控服务工厂
func NewInventorySyncServiceFactory(managementClient *management.ClientManager) *InventorySyncServiceFactory {
	return &InventorySyncServiceFactory{
		managementClient: managementClient,
		logger:           logrus.WithField("component", "TemuInventorySyncServiceFactory"),
	}
}

// CreateInventorySyncService 创建库存监控服务
func (f *InventorySyncServiceFactory) CreateInventorySyncService(
	temuAPIClient client.ClientAPI,
	amazonProcessor *amazon.AmazonProcessor,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
) InventorySyncService {
	f.logger.Info("创建TEMU库存监控服务")

	// 获取必要的客户端
	rawJsonDataClient := f.managementClient.GetRawJsonDataAdapter()
	inventoryRecordClient := f.managementClient.GetInventoryRecordClient()

	// 创建服务实例
	service := NewInventorySyncService(
		f.managementClient,
		temuAPIClient,
		amazonProcessor,
		amazonConfig,
		monitorConfig,
		rawJsonDataClient,
		inventoryRecordClient,
	)

	f.logger.Info("TEMU库存监控服务创建完成")
	return service
}

// ValidateConfig 验证配置
func (f *InventorySyncServiceFactory) ValidateConfig(
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
) error {
	f.logger.Info("验证TEMU库存监控配置")

	if amazonConfig == nil {
		return fmt.Errorf("Amazon配置不能为空")
	}

	if amazonConfig.PoolSize <= 0 {
		f.logger.Warn("Amazon浏览器池大小未配置，使用默认值3")
	}

	if monitorConfig == nil {
		f.logger.Warn("监控配置未提供，将使用默认配置")
	} else {
		if monitorConfig.PriceChangeThreshold <= 0 {
			f.logger.Warn("价格变化阈值未配置，使用默认值5%")
		}
		if monitorConfig.StockChangeThreshold <= 0 {
			f.logger.Warn("库存变化阈值未配置，使用默认值10")
		}
	}

	f.logger.Info("TEMU库存监控配置验证完成")
	return nil
}
