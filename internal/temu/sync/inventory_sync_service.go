// package sync 提供TEMU平台库存监控服务实现
package sync

import (
	"context"
	"fmt"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/pricing"
	"task-processor/internal/product"
	"task-processor/internal/temu/api/client"

	"github.com/sirupsen/logrus"
)

// inventorySyncServiceImpl TEMU库存监控服务实现
type inventorySyncServiceImpl struct {
	runtime               inventorySyncRuntime
	temuAPIClient         client.ClientAPI
	productFetcher        fetcher.ProductFetcher
	rawJsonDataClient     product.RawJsonDataClient
	inventoryRecordClient managementapi.InventoryRecordAPI
	storeRepo             temuInventoryStoreFinder
	productDataRepo       listingadmin.ProductDataRepository
	monitorConfig         *config.MonitorConfig
	costCalculator        *pricing.CostCalculator
	logger                *logrus.Entry
}

type temuInventoryStoreFinder interface {
	FindStoreByID(ctx context.Context, id int64) (*listingadmin.Store, error)
}

type inventorySyncRuntime interface {
	pricing.OperationStrategyProvider
	productDataClientFactory
	GetStoreAPI() managementapi.StoreAPI
	GetLocalStoreRepository() *listingadmin.GormStoreRepository
	GetLocalProductDataRepository() listingadmin.ProductDataRepository
}

// NewInventorySyncService 创建TEMU库存监控服务
func NewInventorySyncService(
	runtime inventorySyncRuntime,
	temuAPIClient client.ClientAPI,
	productFetcher fetcher.ProductFetcher,
	monitorConfig *config.MonitorConfig,
	rawJsonDataClient product.RawJsonDataClient,
	inventoryRecordClient managementapi.InventoryRecordAPI,
) InventorySyncService {
	log := logger.GetGlobalLogger("TemuInventorySyncService")

	// 创建通用成本计算器（TEMU不需要详细日志）
	costCalculator := pricing.NewCostCalculator(runtime, log, false)

	return &inventorySyncServiceImpl{
		runtime:               runtime,
		temuAPIClient:         temuAPIClient,
		productFetcher:        productFetcher,
		rawJsonDataClient:     rawJsonDataClient,
		inventoryRecordClient: inventoryRecordClient,
		storeRepo: func() temuInventoryStoreFinder {
			if runtime == nil {
				return nil
			}
			return runtime.GetLocalStoreRepository()
		}(),
		productDataRepo: func() listingadmin.ProductDataRepository {
			if runtime == nil {
				return nil
			}
			return runtime.GetLocalProductDataRepository()
		}(),
		monitorConfig:  monitorConfig,
		costCalculator: costCalculator,
		logger:         log,
	}
}

// FetchProductsForInventorySync 获取需要监控库存的产品列表
func (s *inventorySyncServiceImpl) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*TemuInventoryProductSnapshot, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Info("开始获取需要监控库存的TEMU产品")

	if s.productDataRepo != nil {
		shelfStatus := managementapi.ShelfStatusOnShelf
		page, err := s.productDataRepo.ListProductData(ctx, listingadmin.ProductDataQuery{
			TenantID:    tenantID,
			StoreID:     temuSyncPtrInt64(storeID),
			Platform:    "TEMU",
			ShelfStatus: &shelfStatus,
			Page:        1,
			PageSize:    2000,
		})
		if err != nil {
			return nil, fmt.Errorf("通过产品数据仓储获取TEMU产品列表失败: %w", err)
		}
		products := make([]*TemuInventoryProductSnapshot, 0)
		if page != nil {
			products = make([]*TemuInventoryProductSnapshot, 0, len(page.Items))
			for i := range page.Items {
				products = append(products, temuInventoryProductSnapshotFromProductData(&page.Items[i]))
			}
		}
		s.logger.WithField("count", len(products)).Info("通过产品数据仓储获取需要监控库存的TEMU产品完成")
		return products, nil
	}

	// 从管理系统获取TEMU平台的在售产品
	if s.runtime == nil {
		return nil, fmt.Errorf("inventory sync runtime is not initialized")
	}
	productDataAPI := s.runtime.GetProductDataClient(storeID)
	if productDataAPI == nil {
		return nil, fmt.Errorf("product data client is not initialized for store %d", storeID)
	}
	shelfStatus := managementapi.ShelfStatusOnShelf
	productDTOs, err := productDataAPI.ListByStore("TEMU", tenantID, storeID, &shelfStatus)
	if err != nil {
		s.logger.WithError(err).Warn("从管理系统获取TEMU产品列表失败")
		return nil, fmt.Errorf("获取TEMU产品列表失败: %w", err)
	}

	products := make([]*TemuInventoryProductSnapshot, 0, len(productDTOs))
	for _, prod := range productDTOs {
		products = append(products, temuInventoryProductSnapshotFromDTO(prod))
	}

	s.logger.WithField("count", len(products)).Info("获取需要监控库存的TEMU产品完成")
	return products, nil
}

// MonitorInventoryChanges 监控库存和价格变化（并发处理）
func (s *inventorySyncServiceImpl) MonitorInventoryChanges(ctx context.Context, products []*TemuInventoryProductSnapshot, tenantID, storeID int64) (*MonitorResult, error) {
	totalCount := len(products)
	s.logger.WithField("count", totalCount).Info("开始监控TEMU产品库存和价格变化（并发处理）")

	if totalCount == 0 {
		s.logger.Info("没有TEMU产品需要监控")
		return &MonitorResult{}, nil
	}

	// 预先获取店铺的运营策略（避免重复查询）
	operationStrategy, err := s.getOperationStrategy(storeID)
	if err != nil {
		s.logger.WithError(err).WithField("store_id", storeID).Warn("获取TEMU运营策略失败，将使用默认策略")
	}

	if operationStrategy != nil {
		s.logger.WithFields(logrus.Fields{
			"store_id":              storeID,
			"strategy_enabled":      operationStrategy.IsEnabled(),
			"price_increase_action": operationStrategy.PriceIncreaseAction,
			"price_decrease_action": operationStrategy.PriceDecreaseAction,
			"stock_change_action":   operationStrategy.StockChangeAction,
			"out_of_stock_action":   operationStrategy.OutOfStockAction,
		}).Info("成功获取TEMU运营策略")
	} else {
		s.logger.WithField("store_id", storeID).Info("未配置TEMU运营策略，将使用默认策略")
	}

	// 使用并发处理
	return s.monitorInventoryChangesConcurrent(ctx, products, tenantID, storeID, operationStrategy)
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
	}).Infof("TEMU库存监控进度: %d/%d (%.1f%%)", currentIndex, totalCount, progress)
}

// getOperationStrategy 获取运营策略
func (s *inventorySyncServiceImpl) getOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	// 从运行时获取运营策略，优先走本地仓储/运行时抽象
	if s.runtime == nil {
		return nil, fmt.Errorf("inventory sync runtime is not initialized")
	}
	strategy, err := s.runtime.GetRuntimeOperationStrategy(storeID)
	if err != nil {
		return nil, fmt.Errorf("获取TEMU店铺运营策略失败: %w", err)
	}

	// 检查策略是否启用且平台匹配
	if strategy != nil && strategy.IsEnabled() && strategy.Platform == "TEMU" {
		return strategy, nil
	}

	return nil, nil
}
