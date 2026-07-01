// Package inventory 提供SHEIN平台库存同步服务
package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/fetcher"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/recovery"
	"task-processor/internal/pricing"
	"task-processor/internal/product"
	shein_product "task-processor/internal/shein/api/product"
	"task-processor/internal/shein/productsync"

	"github.com/sirupsen/logrus"
)

// InventorySyncService 库存监控服务接口（监控Amazon价格和库存变化）
type InventorySyncService interface {
	// FetchProductsForInventorySync 获取需要监控库存的产品列表
	FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*InventoryProductSnapshot, error)

	// MonitorInventoryChanges 监控库存和价格变化
	MonitorInventoryChanges(ctx context.Context, products []*InventoryProductSnapshot, tenantID, storeID int64) (*MonitorResult, error)
}

// InventoryUpdateRequest 库存更新请求
type InventoryUpdateRequest struct {
	ProductID     string
	PlatformSKU   string
	NewInventory  int
	AmazonProduct *model.Product
	StoreID       int64 // 添加StoreID用于获取价格类型配置
}

// inventorySyncServiceImpl 库存监控服务实现
type inventorySyncServiceImpl struct {
	strategyProvider    pricing.OperationStrategyProvider
	productAPI          shein_product.ProductAPI
	productFetcher      fetcher.ProductFetcher
	rawJsonDataClient   product.RawJsonDataClient
	inventoryRecordRepo listingadmin.InventoryRecordRepository
	storeService        listingruntime.StoreService
	storeRepo           sheinInventoryStoreFinder
	productDataRepo     listingadmin.ProductDataRepository
	syncedProductSource SyncedInventoryProductSource
	monitorConfig       *config.MonitorConfig
	costCalculator      *pricing.CostCalculator
	logger              *logrus.Entry

	// 批量库存更新收集器
	inventoryUpdates sync.Map // map[string][]InventoryUpdateRequest - key: productID
}

type sheinInventoryStoreFinder interface {
	FindStoreByID(ctx context.Context, id int64) (*listingadmin.Store, error)
}

// NewInventorySyncService 创建库存监控服务
func NewInventorySyncService(
	strategyProvider pricing.OperationStrategyProvider,
	productAPI shein_product.ProductAPI,
	productFetcher fetcher.ProductFetcher,
	monitorConfig *config.MonitorConfig,
	rawJsonDataClient product.RawJsonDataClient,
	inventoryRecordRepo listingadmin.InventoryRecordRepository,
	storeService listingruntime.StoreService,
	storeRepo sheinInventoryStoreFinder,
	productDataRepo listingadmin.ProductDataRepository,
) InventorySyncService {
	return NewInventorySyncServiceWithSyncedProductSource(
		strategyProvider,
		productAPI,
		productFetcher,
		monitorConfig,
		rawJsonDataClient,
		inventoryRecordRepo,
		storeService,
		storeRepo,
		productDataRepo,
		nil,
	)
}

func NewInventorySyncServiceWithSyncedProductSource(
	strategyProvider pricing.OperationStrategyProvider,
	productAPI shein_product.ProductAPI,
	productFetcher fetcher.ProductFetcher,
	monitorConfig *config.MonitorConfig,
	rawJsonDataClient product.RawJsonDataClient,
	inventoryRecordRepo listingadmin.InventoryRecordRepository,
	storeService listingruntime.StoreService,
	storeRepo sheinInventoryStoreFinder,
	productDataRepo listingadmin.ProductDataRepository,
	syncedProductSource SyncedInventoryProductSource,
) InventorySyncService {
	// 临时设置 Debug 级别以便调试映射问题
	logger.SetGlobalLogLevel("debug")

	log := logger.GetGlobalLogger("InventorySyncService")

	// 创建通用成本计算器（SHEIN需要详细日志）
	costCalculator := pricing.NewCostCalculator(strategyProvider, log, true)

	return &inventorySyncServiceImpl{
		strategyProvider:    strategyProvider,
		productAPI:          productAPI,
		productFetcher:      productFetcher,
		rawJsonDataClient:   rawJsonDataClient,
		inventoryRecordRepo: inventoryRecordRepo,
		storeService:        storeService,
		storeRepo:           storeRepo,
		productDataRepo:     productDataRepo,
		syncedProductSource: syncedProductSource,
		monitorConfig:       monitorConfig,
		costCalculator:      costCalculator,
		logger:              log,
	}
}

// FetchProductsForInventorySync 获取需要监控库存的产品列表
func (s *inventorySyncServiceImpl) FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*InventoryProductSnapshot, error) {
	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"store_id":  storeID,
	}).Info("开始获取需要监控库存的产品")

	if s.syncedProductSource != nil {
		items, err := s.fetchSyncedProductsForInventorySync(ctx, tenantID, storeID)
		if err != nil {
			return nil, fmt.Errorf("通过SHEIN同步产品仓储获取库存同步产品列表失败: %w", err)
		}
		s.logger.WithField("count", len(items)).Info("通过SHEIN同步产品仓储获取需要监控库存的产品完成")
		return items, nil
	}

	if s.productDataRepo != nil {
		shelfStatus := sheinInventoryShelfStatusOnShelf
		page, err := s.productDataRepo.ListProductData(ctx, listingadmin.ProductDataQuery{
			TenantID:    tenantID,
			StoreID:     sheinInvPtrInt64(storeID),
			Platform:    "SHEIN",
			ShelfStatus: &shelfStatus,
			Page:        1,
			PageSize:    2000,
		})
		if err != nil {
			return nil, fmt.Errorf("通过产品数据仓储获取SHEIN产品列表失败: %w", err)
		}
		items := make([]*InventoryProductSnapshot, 0)
		if page != nil {
			items = make([]*InventoryProductSnapshot, 0, len(page.Items))
			for i := range page.Items {
				items = append(items, inventoryProductSnapshotFromProductData(&page.Items[i]))
			}
		}
		s.logger.WithField("count", len(items)).Info("通过产品数据仓储获取需要监控库存的产品完成")
		return items, nil
	}

	return nil, fmt.Errorf("product data repository is not initialized")
}

// MonitorInventoryChanges 监控库存和价格变化
func (s *inventorySyncServiceImpl) MonitorInventoryChanges(ctx context.Context, products []*InventoryProductSnapshot, tenantID, storeID int64) (*MonitorResult, error) {
	totalCount := len(products)
	s.logger.WithField("count", totalCount).Info("开始监控产品库存和价格变化（并发模式）")

	if totalCount == 0 {
		s.logger.Info("没有产品需要监控")
		return &MonitorResult{}, nil
	}

	result := &MonitorResult{
		TotalProducts: totalCount,
	}

	// 使用互斥锁保护结果统计
	var resultMutex sync.Mutex

	// 使用 WaitGroup 等待所有 goroutine 完成
	var wg sync.WaitGroup

	// 控制并发数量（使用浏览器池大小）
	concurrency := min(totalCount, 1)

	s.logger.Infof("使用并发模式监控库存，并发数: %d (浏览器池大小: %d)", concurrency, 1)

	semaphore := make(chan struct{}, concurrency)

	progressInterval := s.calculateProgressInterval(totalCount)
	var processedCount int32 // 使用原子操作计数

	for i, prod := range products {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量

		go func(_ int, product *InventoryProductSnapshot) {
			defer func() {
				<-semaphore // 释放信号量
				wg.Done()

				// 原子递增已处理数量
				processed := atomic.AddInt32(&processedCount, 1)

				// 输出进度日志
				if int(processed)%progressInterval == 0 || int(processed) == totalCount {
					resultMutex.Lock()
					s.logProgress(int(processed), totalCount, result)
					resultMutex.Unlock()
				}
			}()
			defer recovery.Recover("监控产品库存变化", s.logger)

			// 从 Attributes 中解析所有 SKU 映射数据
			skuMappingList := s.extractMappingInfoFromAttributes(product.Attributes)

			if len(skuMappingList) == 0 {
				resultMutex.Lock()
				result.SkippedProducts++
				resultMutex.Unlock()
				return
			}

			// 遍历所有 SKU 的映射数据
			for _, skuMapping := range skuMappingList {
				if err := s.monitorSingleSKU(ctx, product, skuMapping, tenantID, storeID, result, &resultMutex); err != nil {
					s.logger.WithError(err).WithFields(logrus.Fields{
						"product_id": product.ProductID,
						"sku":        s.getStringValue(skuMapping.MappingInfo.SKU),
					}).Warn("监控SKU失败")
				}
			}

			// 获取当前产品收集到的所有库存更新并立即处理
			if value, exists := s.inventoryUpdates.LoadAndDelete(product.ProductID); exists {
				updates := value.([]InventoryUpdateRequest)
				if len(updates) > 0 {
					// 立即处理当前产品的库存更新（参考TEMU的方式）
					if err := s.processSingleProductInventoryUpdates(ctx, product, updates, storeID); err != nil {
						s.logger.WithError(err).WithField("product_id", product.ProductID).Error("处理产品库存更新失败")
					} else {
						s.logger.WithFields(logrus.Fields{
							"product_id":   product.ProductID,
							"update_count": len(updates),
						}).Debug("成功处理单个产品的库存更新")
					}
				}
			}

			resultMutex.Lock()
			result.ProcessedProducts++
			resultMutex.Unlock()
		}(i, prod)
	}

	// 等待所有 goroutine 完成
	wg.Wait()

	s.logger.WithFields(logrus.Fields{
		"total":          result.TotalProducts,
		"processed":      result.ProcessedProducts,
		"skipped":        result.SkippedProducts,
		"price_changes":  result.PriceChanges,
		"stock_changes":  result.StockChanges,
		"amazon_fetched": result.AmazonFetched,
		"amazon_failed":  result.AmazonFailed,
	}).Info("监控产品库存和价格变化完成（并发模式）")

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

// addInventoryUpdate 添加库存更新请求到批量收集器
func (s *inventorySyncServiceImpl) addInventoryUpdate(productID, platformSKU string, newInventory int, amazonProduct *model.Product, storeID int64) {
	updateReq := InventoryUpdateRequest{
		ProductID:     productID,
		PlatformSKU:   platformSKU,
		NewInventory:  newInventory,
		AmazonProduct: amazonProduct,
		StoreID:       storeID,
	}

	// 获取或创建该产品的更新列表
	value, _ := s.inventoryUpdates.LoadOrStore(productID, make([]InventoryUpdateRequest, 0))
	updates := value.([]InventoryUpdateRequest)
	updates = append(updates, updateReq)
	s.inventoryUpdates.Store(productID, updates)

	s.logger.WithFields(logrus.Fields{
		"product_id":    productID,
		"platform_sku":  platformSKU,
		"new_inventory": newInventory,
	}).Debug("已添加库存更新请求到批量收集器")
}

// processSingleProductInventoryUpdates 处理单个产品的所有SKU库存更新和Amazon监控数据
func (s *inventorySyncServiceImpl) processSingleProductInventoryUpdates(
	_ context.Context,
	prod *InventoryProductSnapshot,
	updates []InventoryUpdateRequest,
	storeID int64,
) error {
	if len(updates) == 0 {
		return nil
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":   prod.ProductID,
		"update_count": len(updates),
	}).Debug("开始处理单个产品的库存和Amazon监控数据更新")

	if prod.Attributes == "" {
		return fmt.Errorf("产品Attributes为空")
	}

	// 解析现有的attributes数据
	var skcList []productsync.EnrichedSkcInfo
	if err := jsonx.UnmarshalString(prod.Attributes, &skcList, "解析产品attributes失败"); err != nil {
		return fmt.Errorf("解析产品attributes失败: %w", err)
	}

	// 获取店铺配置的价格类型（使用第一个更新请求的StoreID）
	priceType := s.getStorePriceType(storeID)

	// 批量更新所有SKU的库存和Amazon监控数据
	updatedCount := 0
	for _, update := range updates {
		for i := range skcList {
			for j := range skcList[i].SkuInfo {
				sku := &skcList[i].SkuInfo[j]
				if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.SKU) == update.PlatformSKU {
					var oldInventory int
					if sku.UsableInventory != nil {
						oldInventory = *sku.UsableInventory
					}

					// 更新库存
					sku.UsableInventory = &update.NewInventory

					// 更新Amazon监控数据
					var currentPrice float64
					var newStock int
					var asin string
					if amazonProduct := update.AmazonProduct; amazonProduct != nil {
						currentPrice = product.GetProductPrice(amazonProduct, priceType)
						newStock = s.extractStockFromProduct(amazonProduct)
						asin = amazonProduct.Asin
						sku.AmazonMonitorData = &AmazonMonitorData{
							ASIN:          asin,
							Price:         currentPrice,
							Stock:         newStock,
							LastCheckTime: time.Now().Unix(),
						}
					}

					updatedCount++

					s.logger.WithFields(logrus.Fields{
						"platform_sku":  update.PlatformSKU,
						"old_inventory": oldInventory,
						"new_inventory": update.NewInventory,
						"amazon_stock":  newStock,
						"amazon_price":  currentPrice,
						"price_type":    priceType,
						"asin":          asin,
					}).Debug("已更新SKU库存和Amazon监控数据")
					break
				}
			}
		}
	}

	if updatedCount == 0 {
		s.logger.WithField("product_id", prod.ProductID).Warn("未找到任何匹配的SKU进行库存更新")
		return fmt.Errorf("未找到匹配的SKU")
	}

	// 序列化更新后的数据
	updatedAttributes, err := json.Marshal(skcList)
	if err != nil {
		return fmt.Errorf("序列化attributes失败: %w", err)
	}

	count, err := s.updateInventoryProductAttributes(context.Background(), prod, string(updatedAttributes))
	if err != nil {
		return fmt.Errorf("更新产品attributes失败: %w", err)
	}

	if count <= 0 {
		return fmt.Errorf("未更新任何产品attributes")
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":    prod.ProductID,
		"updated_count": count,
		"sku_count":     updatedCount,
		"total_updates": len(updates),
	}).Info("成功批量更新SHEIN产品attributes中的库存和Amazon监控数据")

	return nil
}
