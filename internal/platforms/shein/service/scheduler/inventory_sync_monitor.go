// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	managementapi "task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// monitorSingleSKU 监控单个SKU
func (s *inventorySyncServiceImpl) monitorSingleSKU(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *SKUMappingData,
	tenantID, storeID int64,
	result *MonitorResult,
	resultMutex *sync.Mutex,
) error {
	mappingInfo := skuMapping.MappingInfo
	asin := mappingInfo.ProductId
	if asin == "" {
		s.logger.WithField("platform_sku", s.getStringValue(mappingInfo.Sku)).Debug("映射信息中没有ASIN，跳过")
		return nil
	}

	// 检查上次更新时间和库存监控记录，如果小于24小时且有监控记录则跳过
	if prod.UpdateTime != nil && !prod.UpdateTime.IsZero() {
		timeSinceLastUpdate := time.Since(prod.UpdateTime.Time)
		if timeSinceLastUpdate < 24*time.Hour {
			// 检查 Attributes 中是否有 Amazon 监控数据
			hasMonitorData := s.checkHasAmazonMonitorData(prod.Attributes, s.getStringValue(mappingInfo.Sku))

			if hasMonitorData {
				// 有监控记录且在24小时内更新过，跳过
				s.logger.WithFields(logrus.Fields{
					"product_id":         prod.ProductID,
					"asin":               asin,
					"platform_sku":       s.getStringValue(mappingInfo.Sku),
					"last_update_time":   prod.UpdateTime.Format("2006-01-02 15:04:05"),
					"time_since_update":  timeSinceLastUpdate.Round(time.Minute).String(),
					"has_monitor_record": true,
				}).Debug("产品在24小时内已更新且有监控记录，跳过")
				return nil
			} else {
				// 没有监控记录，即使在24小时内更新过也要继续监控（首次监控）
				s.logger.WithFields(logrus.Fields{
					"product_id":         prod.ProductID,
					"asin":               asin,
					"platform_sku":       s.getStringValue(mappingInfo.Sku),
					"last_update_time":   prod.UpdateTime.Format("2006-01-02 15:04:05"),
					"has_monitor_record": false,
				}).Debug("产品在24小时内已更新但无监控记录，继续监控")
			}
		}
	}

	region := mappingInfo.Region
	if region == "" {
		region = "US"
	}

	// 为单个产品设置2分钟超时
	productCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// 使用 ProductFetcher 获取Amazon产品数据（自动处理缓存）
	amazonProduct, err := s.getAmazonProductData(productCtx, asin, region, tenantID, storeID)
	if err != nil {
		// 检查是否是超时错误
		if productCtx.Err() == context.DeadlineExceeded {
			s.logger.WithFields(logrus.Fields{
				"asin":             asin,
				"platform_sku":     s.getStringValue(mappingInfo.Sku),
				"platform_product": prod.ProductID,
			}).Warn("获取Amazon产品信息超时，跳过该产品")
		} else {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"asin":             asin,
				"platform_sku":     s.getStringValue(mappingInfo.Sku),
				"platform_product": prod.ProductID,
			}).Warn("获取Amazon产品信息失败")
		}
		resultMutex.Lock()
		result.AmazonFailed++
		resultMutex.Unlock()
		return err
	}

	resultMutex.Lock()
	result.AmazonFetched++
	resultMutex.Unlock()

	// 异步更新Attributes中的Amazon数据
	go s.updateAttributesWithAmazonData(prod, s.getStringValue(mappingInfo.Sku), amazonProduct, storeID)

	// 异步记录库存和价格历史（每天一次）
	go s.recordInventoryAndPrice(asin, region, amazonProduct, prod, skuMapping, storeID)

	// 检查价格变化
	if s.checkPriceChange(prod, amazonProduct, skuMapping, storeID) {
		resultMutex.Lock()
		result.PriceChanges++
		resultMutex.Unlock()
	}

	// 检查库存变化
	if s.checkStockChange(amazonProduct, skuMapping, storeID) {
		resultMutex.Lock()
		result.StockChanges++
		resultMutex.Unlock()

		// 根据营销策略处理库存变化（异步）
		go func() {
			defer func() {
				if r := recover(); r != nil {
					s.logger.WithField("panic", r).Error("处理库存变化策略时发生panic")
				}
			}()

			if err := s.handleStockChangeWithStrategy(ctx, prod, amazonProduct, skuMapping, storeID); err != nil {
				s.logger.WithError(err).WithFields(logrus.Fields{
					"product_id": prod.ProductID,
					"sku":        s.getStringValue(mappingInfo.Sku),
				}).Error("处理库存变化策略失败")
			}
		}()
	}

	return nil
}

// getAmazonProductData 获取Amazon产品数据（使用ProductFetcher，自动处理缓存）
func (s *inventorySyncServiceImpl) getAmazonProductData(
	ctx context.Context,
	asin, region string,
	tenantID, storeID int64,
) (*model.Product, error) {
	// 使用 ProductFetcher 获取产品（自动处理缓存和爬取）
	fetchReq := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: asin,
		StoreID:   storeID,
		Creator:   "monitor",
	}

	// 为库存监控创建专用的 rawJsonDataClient，设置24小时数据新鲜度
	inventoryRawJsonClient := s.managementClient.GetRawJsonDataClient()
	inventoryRawJsonClient.SetDataFreshnessDays(1) // 24小时 = 1天

	productFetcher := product.NewProductFetcher(
		inventoryRawJsonClient,
		s.amazonConfig,
		s.amazonProcessor,
	)

	// 使用 channel 实现超时控制
	type fetchResult struct {
		product *model.Product
		err     error
	}
	resultChan := make(chan fetchResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.WithField("panic", r).Error("获取Amazon产品时发生panic")
				resultChan <- fetchResult{nil, fmt.Errorf("获取产品时发生panic: %v", r)}
			}
		}()

		amazonProduct, err := productFetcher.FetchProduct(fetchReq)
		resultChan <- fetchResult{amazonProduct, err}
	}()

	// 等待结果或超时
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("获取Amazon产品超时: %w", ctx.Err())
	case result := <-resultChan:
		if result.err != nil {
			return nil, fmt.Errorf("获取Amazon产品失败: %w", result.err)
		}
		return result.product, nil
	}
}

// checkPriceChange 检查价格变化
func (s *inventorySyncServiceImpl) checkPriceChange(
	prod *managementapi.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *SKUMappingData,
	storeID int64,
) bool {
	mappingInfo := skuMapping.MappingInfo
	oldPrice := s.getFloatValue(mappingInfo.CostPrice)
	if oldPrice <= 0 {
		oldPrice = s.parsePrice(prod.OriginalPrice.String())
		if oldPrice <= 0 {
			oldPrice = s.parsePrice(prod.SpecialPrice.String())
		}
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(storeID)

	// 使用公共函数获取新价格
	newPrice := product.GetProductPrice(amazonProduct, priceType)

	if oldPrice > 0 && newPrice > 0 {
		changePercent := ((newPrice - oldPrice) / oldPrice) * 100

		// 获取价格变化阈值（优先使用平台配置）
		threshold := s.getPriceChangeThreshold(storeID)

		if s.abs(changePercent) >= threshold {
			s.logger.WithFields(logrus.Fields{
				"asin":           mappingInfo.ProductId,
				"sku":            s.getStringValue(mappingInfo.Sku),
				"old_price":      oldPrice,
				"new_price":      newPrice,
				"price_type":     priceType,
				"change_percent": changePercent,
				"threshold":      threshold,
			}).Info("检测到价格变化")
			return true
		}
	}
	return false
}

// checkStockChange 检查库存变化
func (s *inventorySyncServiceImpl) checkStockChange(
	amazonProduct *model.Product,
	skuMapping *SKUMappingData,
	storeID int64,
) bool {
	oldStock := skuMapping.Stock
	newStock := s.extractStockFromProduct(amazonProduct)
	changeAmount := newStock - oldStock

	// 获取库存变化阈值（优先使用店铺级策略）
	threshold := s.getStockChangeThreshold(storeID)

	if s.absInt(changeAmount) >= threshold {
		s.logger.WithFields(logrus.Fields{
			"asin":          skuMapping.MappingInfo.ProductId,
			"sku":           s.getStringValue(skuMapping.MappingInfo.Sku),
			"old_stock":     oldStock,
			"new_stock":     newStock,
			"change_amount": changeAmount,
			"threshold":     threshold,
		}).Info("检测到库存变化")
		return true
	}
	return false
}

// getPriceChangeThreshold 获取价格变化阈值（从平台配置）
func (s *inventorySyncServiceImpl) getPriceChangeThreshold(storeID int64) float64 {
	if s.monitorConfig != nil {
		return s.monitorConfig.PriceChangeThreshold
	}
	return 5.0 // 默认5%
}

// getStockChangeThreshold 获取库存变化阈值（优先从店铺级策略获取）
func (s *inventorySyncServiceImpl) getStockChangeThreshold(storeID int64) int {
	// 尝试从管理系统获取店铺级策略
	strategy, err := s.managementClient.GetOperationStrategyClient().GetOperationStrategyByStoreId(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		if strategy.StockChangeThreshold > 0 {
			return strategy.StockChangeThreshold
		}
	}

	// 使用平台配置作为默认值
	if s.monitorConfig != nil {
		return s.monitorConfig.StockChangeThreshold
	}
	return 5 // 默认5个
}
