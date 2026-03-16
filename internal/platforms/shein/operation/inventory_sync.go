// Package operation 提供SHEIN平台调度器相关服务
package operation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/platforms/pricing"
	shein_product "task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// InventorySyncService 库存监控服务接口（监控Amazon价格和库存变化）
type InventorySyncService interface {
	// FetchProductsForInventorySync 获取需要监控库存的产品列表
	FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*managementapi.ProductDataDTO, error)

	// MonitorInventoryChanges 监控库存和价格变化
	MonitorInventoryChanges(ctx context.Context, products []*managementapi.ProductDataDTO, tenantID, storeID int64) (*MonitorResult, error)
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
	managementClient      *management.ClientManager
	productAPI            shein_product.ProductAPI
	amazonProcessor       *amazon.AmazonProcessor
	amazonConfig          *config.AmazonConfig
	rawJsonDataClient     product.RawJsonDataClient
	inventoryRecordClient managementapi.InventoryRecordAPI
	monitorConfig         *config.MonitorConfig
	costCalculator        *pricing.CostCalculator
	logger                *logrus.Entry

	// 批量库存更新收集器
	inventoryUpdates sync.Map // map[string][]InventoryUpdateRequest - key: productID
}

// NewInventorySyncService 创建库存监控服务
func NewInventorySyncService(
	managementClient *management.ClientManager,
	productAPI shein_product.ProductAPI,
	amazonProcessor *amazon.AmazonProcessor,
	amazonConfig *config.AmazonConfig,
	monitorConfig *config.MonitorConfig,
	rawJsonDataClient product.RawJsonDataClient,
	inventoryRecordClient managementapi.InventoryRecordAPI,
) InventorySyncService {
	// 临时设置 Debug 级别以便调试映射问题
	logrus.SetLevel(logrus.DebugLevel)

	logger := logrus.WithField("component", "InventorySyncService")

	// 创建通用成本计算器（SHEIN需要详细日志）
	costCalculator := pricing.NewCostCalculator(managementClient, logger, true)

	return &inventorySyncServiceImpl{
		managementClient:      managementClient,
		productAPI:            productAPI,
		amazonProcessor:       amazonProcessor,
		amazonConfig:          amazonConfig,
		rawJsonDataClient:     rawJsonDataClient,
		inventoryRecordClient: inventoryRecordClient,
		monitorConfig:         monitorConfig,
		costCalculator:        costCalculator,
		logger:                logger,
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
	concurrency := s.amazonConfig.PoolSize
	if concurrency <= 0 {
		concurrency = 3 // 默认值
	}
	if totalCount < concurrency {
		concurrency = totalCount
	}

	s.logger.Infof("使用并发模式监控库存，并发数: %d (浏览器池大小: %d)", concurrency, s.amazonConfig.PoolSize)

	semaphore := make(chan struct{}, concurrency)

	progressInterval := s.calculateProgressInterval(totalCount)
	var processedCount int32 // 使用原子操作计数

	for i, prod := range products {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量

		go func(index int, product *managementapi.ProductDataDTO) {
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
						"sku":        s.getStringValue(skuMapping.MappingInfo.Sku),
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

// processBatchInventoryUpdates 批量处理所有收集到的库存更新
func (s *inventorySyncServiceImpl) processBatchInventoryUpdates(ctx context.Context, products []*managementapi.ProductDataDTO, storeID int64) {
	s.logger.Info("开始批量处理库存更新")

	// 创建产品ID到产品对象的映射
	productMap := make(map[string]*managementapi.ProductDataDTO)
	for _, prod := range products {
		productMap[prod.ProductID] = prod
	}

	updateCount := 0
	s.inventoryUpdates.Range(func(key, value any) bool {
		productID := key.(string)
		updates := value.([]InventoryUpdateRequest)

		if len(updates) == 0 {
			return true
		}

		// 从映射中获取产品数据
		prod, exists := productMap[productID]
		if !exists {
			s.logger.WithField("product_id", productID).Warn("未找到产品数据，跳过库存更新")
			return true
		}

		// 处理单个产品的所有SKU库存更新
		if err := s.processSingleProductInventoryUpdates(ctx, prod, updates, storeID); err != nil {
			s.logger.WithError(err).WithField("product_id", productID).Error("批量更新产品库存失败")
		} else {
			updateCount += len(updates)
		}

		return true
	})

	// 清空收集器
	s.inventoryUpdates = sync.Map{}

	s.logger.WithField("update_count", updateCount).Info("批量库存更新处理完成")
}

// processSingleProductInventoryUpdates 处理单个产品的所有SKU库存更新和Amazon监控数据
func (s *inventorySyncServiceImpl) processSingleProductInventoryUpdates(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
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
	var skcList []EnrichedSkcInfo
	if err := jsonutil.UnmarshalString(prod.Attributes, &skcList, "解析产品attributes失败"); err != nil {
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
				if sku.MappingInfo != nil && s.getStringValue(sku.MappingInfo.Sku) == update.PlatformSKU {
					var oldInventory int
					if sku.UsableInventory != nil {
						oldInventory = *sku.UsableInventory
					}

					// 更新库存
					sku.UsableInventory = &update.NewInventory

					// 更新Amazon监控数据
					amazonProduct := update.AmazonProduct
					currentPrice := product.GetProductPrice(amazonProduct, priceType)
					newStock := s.extractStockFromProduct(amazonProduct)

					sku.AmazonMonitorData = &AmazonMonitorData{
						ASIN:          amazonProduct.Asin,
						Price:         currentPrice,
						Stock:         newStock,
						LastCheckTime: time.Now().Unix(),
					}

					updatedCount++

					s.logger.WithFields(logrus.Fields{
						"platform_sku":  update.PlatformSKU,
						"old_inventory": oldInventory,
						"new_inventory": update.NewInventory,
						"amazon_stock":  newStock,
						"amazon_price":  currentPrice,
						"price_type":    priceType,
						"asin":          amazonProduct.Asin,
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

	// 使用批量更新attributes接口
	productDataAPI := s.managementClient.GetProductDataClient(storeID)
	updateReq := &managementapi.ProductDataBatchUpdateAttributesReqDTO{
		Platform: "SHEIN",
		TenantID: prod.TenantID,
		StoreID:  storeID,
		Region:   prod.Region,
		Products: []managementapi.ProductAttributesItemDTO{
			{
				PlatformProductID: prod.PlatformProductID,
				Attributes:        string(updatedAttributes),
				UpdateTime:        &[]int64{time.Now().Unix()}[0],
			},
		},
	}

	count, err := productDataAPI.BatchUpdateAttributes(updateReq)
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

