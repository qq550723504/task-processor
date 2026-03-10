// Package scheduler 提供TEMU平台库存监控核心逻辑
package scheduler

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/pkg/contextutil"
	managementapi "task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// monitorSingleProduct 监控单个TEMU产品（支持并发处理）
func (s *inventorySyncServiceImpl) monitorSingleProduct(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	tenantID, storeID int64,
	result *MonitorResult,
	resultMutex *sync.Mutex, // 并发模式下不为nil，顺序模式下为nil
	operationStrategy *managementapi.OperationStrategyDTO, // 预获取的运营策略
) error {
	s.logger.WithField("product_id", prod.ProductID).Debug("开始监控单个TEMU产品")

	// 从 Attributes 中解析所有 SKU 映射数据
	skuMappingList := s.extractMappingInfoFromAttributes(prod.Attributes)

	if len(skuMappingList) == 0 {
		s.logger.WithField("product_id", prod.ProductID).Debug("TEMU产品没有映射信息，跳过")
		// 更新跳过计数
		if resultMutex != nil {
			resultMutex.Lock()
			result.SkippedProducts++
			resultMutex.Unlock()
		} else {
			result.SkippedProducts++
		}
		return nil
	}

	// 收集所有需要更新库存的SKU信息
	var inventoryUpdates []SkuInventoryUpdate

	// 遍历所有 SKU 的映射数据
	for _, skuMapping := range skuMappingList {
		update, err := s.monitorSingleSKU(ctx, prod, skuMapping, tenantID, storeID, result, resultMutex, operationStrategy)
		if err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"product_id": prod.ProductID,
				"sku":        s.getStringValue(skuMapping.SkuInfo[0].MappingInfo.Sku),
			}).Error("监控TEMU SKU失败")
			continue
		}

		// 如果有库存更新需求，添加到批量更新列表
		if update != nil {
			inventoryUpdates = append(inventoryUpdates, *update)
		}
	}

	// 批量更新库存
	if len(inventoryUpdates) > 0 {
		batch := &InventoryUpdateBatch{
			Product: prod,
			Updates: inventoryUpdates,
			StoreID: storeID,
		}

		if err := s.batchUpdateTemuInventoryInAttributes(ctx, batch); err != nil {
			s.logger.WithError(err).WithField("product_id", prod.ProductID).Error("批量更新TEMU产品库存失败")
		}
	}

	return nil
}

// monitorSingleSKU 监控单个SKU（支持并发处理）
// 返回库存更新信息，如果不需要更新则返回nil
func (s *inventorySyncServiceImpl) monitorSingleSKU(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuMappingData,
	tenantID, storeID int64,
	result *MonitorResult,
	resultMutex *sync.Mutex, // 并发模式下不为nil，顺序模式下为nil
	operationStrategy *managementapi.OperationStrategyDTO, // 预获取的运营策略
) (*SkuInventoryUpdate, error) {
	mappingInfo := skuMapping.SkuInfo[0].MappingInfo
	asin := mappingInfo.ProductId
	if asin == "" {
		s.logger.WithField("platform_sku", s.getStringValue(mappingInfo.Sku)).Debug("映射信息中没有ASIN，跳过")
		return nil, nil
	}

	// 检查Amazon监控数据的最后检查时间，如果小于24小时则跳过
	lastCheckTime := s.getAmazonMonitorLastCheckTime(prod.Attributes, s.getStringValue(mappingInfo.Sku))
	if lastCheckTime > 0 {
		lastCheckTimeObj := time.Unix(lastCheckTime, 0)
		timeSinceLastCheck := time.Since(lastCheckTimeObj)
		if timeSinceLastCheck < 24*time.Hour {
			// Amazon监控数据在24小时内已检查过，跳过
			s.logger.WithFields(logrus.Fields{
				"platform_sku":     s.getStringValue(mappingInfo.Sku),
				"last_check_time":  lastCheckTime,
				"time_since_check": timeSinceLastCheck.String(),
			}).Debug("Amazon监控数据在24小时内已检查过，跳过")
			return nil, nil
		}
	}

	region := mappingInfo.Region
	if region == "" {
		region = "US"
	}

	// 为单个产品设置2分钟超时
	productCtx, cancel := contextutil.WithTaskTimeout(ctx)
	defer cancel()

	s.logger.WithFields(logrus.Fields{
		"asin":         asin,
		"region":       region,
		"platform_sku": s.getStringValue(mappingInfo.Sku),
	}).Info("开始获取Amazon产品数据")

	// 使用 ProductFetcher 获取Amazon产品数据
	amazonProduct, err := s.getAmazonProductData(productCtx, asin, region, tenantID, storeID)
	if err != nil {
		// 检查是否是超时错误
		if productCtx.Err() == context.DeadlineExceeded {
			s.logger.WithFields(logrus.Fields{
				"asin":             asin,
				"platform_sku":     s.getStringValue(mappingInfo.Sku),
				"platform_product": prod.ProductID,
			}).Error("获取Amazon产品信息超时，跳过该产品")
		} else {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"asin":             asin,
				"platform_sku":     s.getStringValue(mappingInfo.Sku),
				"platform_product": prod.ProductID,
			}).Error("获取Amazon产品信息失败")
		}
		// 更新失败计数
		if resultMutex != nil {
			resultMutex.Lock()
			result.AmazonFailed++
			resultMutex.Unlock()
		} else {
			result.AmazonFailed++
		}
		return nil, err
	}

	// 更新成功计数
	if resultMutex != nil {
		resultMutex.Lock()
		result.AmazonFetched++
		resultMutex.Unlock()
	} else {
		result.AmazonFetched++
	}

	// 记录库存和价格历史（每天一次）
	go s.recordInventoryAndPrice(asin, region, amazonProduct, prod, &skuMapping.SkuInfo[0], storeID)

	// 检查价格变化
	if s.checkPriceChange(prod, amazonProduct, &skuMapping.SkuInfo[0], storeID) {
		s.logger.WithFields(logrus.Fields{
			"asin":         asin,
			"platform_sku": s.getStringValue(mappingInfo.Sku),
		}).Info("检测到价格变化")

		// 更新价格变化计数
		if resultMutex != nil {
			resultMutex.Lock()
			result.PriceChanges++
			resultMutex.Unlock()
		} else {
			result.PriceChanges++
		}

		// 根据价格变化策略处理库存（同步执行，便于调试）
		s.logger.Debug("开始处理价格变化策略")
		if err := s.handlePriceChangeWithStrategy(ctx, prod, amazonProduct, &skuMapping.SkuInfo[0], operationStrategy); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"product_id": prod.ProductID,
				"sku":        s.getStringValue(&skuMapping.SkuInfo[0].SkuCode),
			}).Error("处理TEMU价格变化策略失败")
		}
	}

	// 检查库存变化
	if s.checkStockChange(amazonProduct, &skuMapping.SkuInfo[0], storeID) {
		s.logger.WithFields(logrus.Fields{
			"asin":         asin,
			"platform_sku": s.getStringValue(mappingInfo.Sku),
		}).Info("检测到库存变化")

		// 更新库存变化计数
		if resultMutex != nil {
			resultMutex.Lock()
			result.StockChanges++
			resultMutex.Unlock()
		} else {
			result.StockChanges++
		}

		// 根据营销策略处理库存变化（同步执行，便于调试）
		s.logger.Debug("开始处理库存变化策略")
		if err := s.handleStockChangeWithStrategy(ctx, prod, amazonProduct, &skuMapping.SkuInfo[0], operationStrategy); err != nil {
			s.logger.WithError(err).WithFields(logrus.Fields{
				"product_id": prod.ProductID,
				"sku":        s.getStringValue(mappingInfo.Sku),
			}).Error("处理TEMU库存变化策略失败")
		}
	}

	// 返回库存更新信息，用于批量更新
	return &SkuInventoryUpdate{
		PlatformSKU:   s.getStringValue(mappingInfo.Sku),
		NewInventory:  int64(s.extractStockFromProduct(amazonProduct)),
		AmazonProduct: amazonProduct,
		SkuInfo:       &skuMapping.SkuInfo[0],
		StoreID:       storeID,
	}, nil
}
