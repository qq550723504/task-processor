// Package operation 提供SHEIN平台调度器相关服务
package operation

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/pkg/contextutil"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/recovery"

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

	// 检查Amazon监控数据的最后检查时间，如果小于24小时则跳过
	lastCheckTime := s.getAmazonMonitorLastCheckTime(prod.Attributes, s.getStringValue(mappingInfo.Sku))
	if lastCheckTime > 0 {
		lastCheckTimeObj := time.Unix(lastCheckTime, 0)
		timeSinceLastCheck := time.Since(lastCheckTimeObj)
		if timeSinceLastCheck < 24*time.Hour {
			// Amazon监控数据在24小时内已检查过，跳过
			return nil
		}
	}

	region := mappingInfo.Region
	if region == "" {
		region = "US"
	}

	// 为单个产品设置2分钟超时
	productCtx, cancel := contextutil.WithTaskTimeout(ctx)
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

	// 记录库存和价格历史（每天一次）
	go s.recordInventoryAndPrice(asin, region, amazonProduct, prod, skuMapping, storeID)

	// 检查价格变化
	if s.checkPriceChange(prod, amazonProduct, skuMapping, storeID) {
		resultMutex.Lock()
		result.PriceChanges++
		resultMutex.Unlock()

		// 根据价格变化策略处理库存（异步）
		go func() {
			defer recovery.Recover("处理价格变化策略", s.logger)

			if err := s.handlePriceChangeWithStrategy(ctx, prod, amazonProduct, skuMapping, storeID); err != nil {
				s.logger.WithError(err).WithFields(logrus.Fields{
					"product_id": prod.ProductID,
					"sku":        s.getStringValue(skuMapping.MappingInfo.Sku),
				}).Error("处理价格变化策略失败")
			}
		}()
	}

	// 检查库存变化
	if s.checkStockChange(amazonProduct, skuMapping, storeID) {
		resultMutex.Lock()
		result.StockChanges++
		resultMutex.Unlock()

		// 根据营销策略处理库存变化（异步）
		go func() {
			defer recovery.Recover("处理库存变化策略", s.logger)

			if err := s.handleStockChangeWithStrategy(ctx, prod, amazonProduct, skuMapping, storeID); err != nil {
				s.logger.WithError(err).WithFields(logrus.Fields{
					"product_id": prod.ProductID,
					"sku":        s.getStringValue(mappingInfo.Sku),
				}).Error("处理库存变化策略失败")
			}
		}()
	}

	// 更新管理系统中的产品库存（添加到批量收集器）
	newInventory := s.extractStockFromProduct(amazonProduct)
	s.addInventoryUpdate(prod.ProductID, s.getStringValue(skuMapping.MappingInfo.Sku), newInventory, amazonProduct, storeID)

	return nil
}
