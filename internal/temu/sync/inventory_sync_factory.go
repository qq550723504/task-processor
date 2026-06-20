// package sync 提供TEMU库存监控服务工厂
package sync

import (
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/fetcher"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/ports"
	"task-processor/internal/product"
	"task-processor/internal/temu/api/client"

	"github.com/sirupsen/logrus"
)

// InventorySyncServiceFactory TEMU库存监控服务工厂
type InventorySyncServiceFactory struct {
	runtime inventoryServiceFactoryRuntime
	logger  *logrus.Entry
}

type inventoryServiceFactoryRuntime interface {
	inventorySyncRuntime
	GetRawJsonDataAdapter() product.RawJsonDataClient
	GetInventoryRecordAPI() managementapi.InventoryRecordAPI
}

// NewInventorySyncServiceFactory 创建库存监控服务工厂
func NewInventorySyncServiceFactory(runtime inventoryServiceFactoryRuntime) *InventorySyncServiceFactory {
	return &InventorySyncServiceFactory{
		runtime: runtime,
		logger:  logger.GetGlobalLogger("TemuInventorySyncServiceFactory"),
	}
}

// CreateInventorySyncService 创建库存监控服务
func (f *InventorySyncServiceFactory) CreateInventorySyncService(
	temuAPIClient client.ClientAPI,
	crawlSource ports.CrawlSource,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rabbitmqClient *rabbitmq.Client,
) (InventorySyncService, error) {
	f.logger.Info("创建TEMU库存监控服务")

	if f.runtime == nil {
		return nil, fmt.Errorf("inventory service runtime is not initialized")
	}
	rawJsonDataClient := f.runtime.GetRawJsonDataAdapter()
	inventoryRecordClient := f.runtime.GetInventoryRecordAPI()

	productFetcher, err := fetcher.NewFetcherFactory().CreateFetcher(
		fetcher.DistributedFetcher,
		rawJsonDataClient,
		amazonConfig,
		crawlSource,
		rabbitmqClient,
	)
	if err != nil {
		return nil, fmt.Errorf("create distributed product fetcher: %w", err)
	}

	service := NewInventorySyncService(
		f.runtime,
		temuAPIClient,
		productFetcher,
		monitorConfig,
		rawJsonDataClient,
		inventoryRecordClient,
	)

	f.logger.Info("TEMU库存监控服务创建完成")
	return service, nil
}

// ValidateConfig 验证配置
func (f *InventorySyncServiceFactory) ValidateConfig(
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
) error {
	if amazonConfig == nil {
		return fmt.Errorf("Amazon配置不能为空")
	}

	if monitorConfig == nil {
		f.logger.Warn("监控配置未提供，将使用默认配置")
	} else {
		if monitorConfig.PriceChangeThreshold <= 0 {
			f.logger.Warn("价格变化阈值未配置，使用默认值5%%")
		}
		if monitorConfig.StockChangeThreshold <= 0 {
			f.logger.Warn("库存变化阈值未配置，使用默认值10")
		}
	}

	return nil
}
