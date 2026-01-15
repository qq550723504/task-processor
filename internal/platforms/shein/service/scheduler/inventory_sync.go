// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein/repo"

	"github.com/sirupsen/logrus"
)

// InventorySyncService 库存监控服务接口（监控Amazon价格和库存变化）
type InventorySyncService interface {
	// FetchProductsForInventorySync 获取需要监控库存的产品列表
	FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)

	// MonitorInventoryChanges 监控库存和价格变化
	MonitorInventoryChanges(ctx context.Context, products []*managementapi.ProductDataDTO, tenantID, storeID int64) (*MonitorResult, error)
}

// inventorySyncServiceImpl 库存监控服务实现
type inventorySyncServiceImpl struct {
	managementClient      *management.ClientManager
	productAPI            repo.ProductAPIInterface
	amazonProcessor       *amazon.AmazonProcessor
	amazonConfig          *config.AmazonConfig
	rawJsonDataClient     managementapi.RawJsonDataAPI
	inventoryRecordClient managementapi.InventoryRecordAPI
	monitorConfig         *config.MonitorConfig // 平台级监控配置（默认值）
	logger                *logrus.Entry
}

// NewInventorySyncService 创建库存监控服务
func NewInventorySyncService(
	managementClient *management.ClientManager,
	productAPI repo.ProductAPIInterface,
	amazonProcessor *amazon.AmazonProcessor,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rawJsonDataClient managementapi.RawJsonDataAPI,
	inventoryRecordClient managementapi.InventoryRecordAPI,
) InventorySyncService {
	return &inventorySyncServiceImpl{
		managementClient:      managementClient,
		productAPI:            productAPI,
		amazonProcessor:       amazonProcessor,
		amazonConfig:          amazonConfig,
		rawJsonDataClient:     rawJsonDataClient,
		inventoryRecordClient: inventoryRecordClient,
		monitorConfig:         monitorConfig,
		logger:                logrus.WithField("component", "InventorySyncService"),
	}
}

// FetchProductsForInventorySync 获取需要监控库存的产品列表
func (s *inventorySyncServiceImpl) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Info("开始获取需要监控库存的产品")

	// 从SHEIN接口获取
	productDataAPI := s.managementClient.GetProductDataClient(storeID)
	shelfStatus := managementapi.ShelfStatusOnShelf
	products, err := productDataAPI.ListByStore("SHEIN", tenantID, storeID, &shelfStatus)
	if err != nil {
		s.logger.WithError(err).Warn("从管理系统获取产品列表失败")
	}

	s.logger.WithField("count", len(products)).Info("获取需要监控库存的产品完成")
	return products, nil
}

// MonitorInventoryChanges 监控库存和价格变化
func (s *inventorySyncServiceImpl) MonitorInventoryChanges(ctx context.Context, products []*managementapi.ProductDataDTO, tenantID, storeID int64) (*MonitorResult, error) {
	totalCount := len(products)
	s.logger.WithField("count", totalCount).Info("开始监控产品库存和价格变化")

	if totalCount == 0 {
		s.logger.Info("没有产品需要监控")
		return &MonitorResult{}, nil
	}

	result := &MonitorResult{
		TotalProducts: totalCount,
	}

	progressInterval := s.calculateProgressInterval(totalCount)

	for i, prod := range products {
		// 从 Attributes 中解析所有 SKU 映射数据
		skuMappingList := s.extractMappingInfoFromAttributes(prod.Attributes)
		if len(skuMappingList) == 0 {
			s.logger.WithField("product_id", prod.ProductID).Debug("产品没有映射信息，跳过")
			result.SkippedProducts++
			continue
		}

		// 遍历所有 SKU 的映射数据
		for _, skuMapping := range skuMappingList {
			if err := s.monitorSingleSKU(ctx, prod, skuMapping, tenantID, storeID, result); err != nil {
				s.logger.WithError(err).WithFields(logrus.Fields{
					"product_id": prod.ProductID,
					"sku":        s.getStringValue(skuMapping.MappingInfo.Sku),
				}).Warn("监控SKU失败")
			}
		}

		result.ProcessedProducts++

		// 输出进度日志
		currentIndex := i + 1
		if currentIndex%progressInterval == 0 || currentIndex == totalCount {
			s.logProgress(currentIndex, totalCount, result)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"total":          result.TotalProducts,
		"processed":      result.ProcessedProducts,
		"skipped":        result.SkippedProducts,
		"price_changes":  result.PriceChanges,
		"stock_changes":  result.StockChanges,
		"amazon_fetched": result.AmazonFetched,
		"amazon_failed":  result.AmazonFailed,
	}).Info("监控产品库存和价格变化完成")

	return result, nil
}

// calculateProgressInterval 计算进度日志间隔
func (s *inventorySyncServiceImpl) calculateProgressInterval(totalCount int) int {
	progressInterval := totalCount / 10
	if progressInterval < 10 {
		progressInterval = 10
	}
	if progressInterval > 100 {
		progressInterval = 100
	}
	return progressInterval
}

// logProgress 输出进度日志
func (s *inventorySyncServiceImpl) logProgress(currentIndex, totalCount int, result *MonitorResult) {
	progress := float64(currentIndex) / float64(totalCount) * 100
	s.logger.WithFields(logrus.Fields{
		"processed":      currentIndex,
		"total":          totalCount,
		"progress":       fmt.Sprintf("%.1f%%", progress),
		"price_changes":  result.PriceChanges,
		"stock_changes":  result.StockChanges,
		"amazon_fetched": result.AmazonFetched,
		"amazon_failed":  result.AmazonFailed,
	}).Infof("库存监控进度: %d/%d (%.1f%%)", currentIndex, totalCount, progress)
}
