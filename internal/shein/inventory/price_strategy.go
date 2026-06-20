// Package inventory 提供 SHEIN 平台库存同步功能
package inventory

import (
	"context"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// handlePriceChangeWithStrategy 根据价格变化策略处理库存（基于利润率）
func (s *inventorySyncServiceImpl) handlePriceChangeWithStrategy(
	ctx context.Context,
	prod *InventoryProductSnapshot,
	amazonProduct *model.Product,
	skuMapping *SKUMappingData,
	storeID int64,
) error {
	// 获取运营策略
	strategy, err := s.strategyProvider.GetRuntimeOperationStrategy(storeID)
	if err != nil {
		s.logger.WithError(err).Debug("获取运营策略失败，跳过价格变化处理")
		return nil
	}

	// 检查策略是否启用
	if strategy == nil || !strategy.IsEnabled() {
		s.logger.Debug("运营策略未启用，跳过价格变化处理")
		return nil
	}

	mappingInfo := skuMapping.MappingInfo

	// 使用成本计算函数获取旧成本价格（包含固定成本）
	oldCostPrice := s.getProductCostPrice(prod, skuMapping, storeID)
	if oldCostPrice <= 0 {
		s.logger.Debug("无法获取旧成本价格，跳过价格变化处理")
		return nil
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(storeID)

	// 使用成本计算函数获取Amazon新成本价格（包含固定成本）
	newCostPrice := s.getAmazonProductCostPrice(amazonProduct, priceType, storeID)
	if newCostPrice <= 0 {
		s.logger.Debug("无法获取Amazon新成本价格，跳过价格变化处理")
		return nil
	}

	// 获取SHEIN产品的销售价格
	sheinSalePrice := s.parsePrice(prod.OriginalPrice.String())
	if sheinSalePrice <= 0 {
		sheinSalePrice = s.parsePrice(prod.SpecialPrice.String())
	}

	if sheinSalePrice <= 0 {
		s.logger.Debug("无法获取SHEIN销售价格，跳过价格变化处理")
		return nil
	}

	// 计算当前利润率和新利润率
	oldProfitRate := (sheinSalePrice - oldCostPrice) / sheinSalePrice
	newProfitRate := (sheinSalePrice - newCostPrice) / sheinSalePrice

	// 获取最低利润率阈值
	minProfitRate := strategy.MinProfitRate
	if minProfitRate <= 0 {
		minProfitRate = 0.15 // 默认15%
	}

	s.logger.WithFields(logrus.Fields{
		"asin":             mappingInfo.ProductID,
		"sku":              s.getStringValue(mappingInfo.SKU),
		"old_cost_price":   oldCostPrice,
		"new_cost_price":   newCostPrice,
		"shein_sale_price": sheinSalePrice,
		"old_profit_rate":  oldProfitRate,
		"new_profit_rate":  newProfitRate,
		"min_profit_rate":  minProfitRate,
	}).Info("开始处理价格变化")

	// 判断利润率变化类型并执行相应策略
	if newProfitRate < minProfitRate && oldProfitRate >= minProfitRate {
		// 利润率从正常降到低于阈值，执行低利润率策略
		return s.handleLowProfitRate(ctx, prod, skuMapping, strategy, storeID, newProfitRate)
	} else if oldProfitRate < minProfitRate && newProfitRate >= minProfitRate {
		// 利润率从低于阈值恢复到正常，执行恢复策略
		return s.handleProfitRateRestore(ctx, prod, skuMapping, strategy, storeID, newProfitRate)
	}

	return nil
}

// handleLowProfitRate 处理低利润率情况
func (s *inventorySyncServiceImpl) handleLowProfitRate(
	ctx context.Context,
	prod *InventoryProductSnapshot,
	skuMapping *SKUMappingData,
	strategy *listingruntime.OperationStrategy,
	storeID int64,
	profitRate float64,
) error {
	action := strategy.LowProfitAction
	if action == "" {
		action = "NONE" // 默认设置库存为0
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":  prod.ProductID,
		"spu_name":    prod.PlatformProductID,
		"sku":         s.getStringValue(skuMapping.MappingInfo.SKU),
		"asin":        skuMapping.MappingInfo.ProductID,
		"profit_rate": profitRate,
		"min_profit":  strategy.MinProfitRate,
		"action":      action,
	}).Info("检测到低利润率，执行策略")

	switch action {
	case "SET_ZERO_STOCK":
		// 将库存设为0，避免亏损销售
		return s.updateProductStockViaSHEINAPI(ctx, prod, skuMapping, 0, storeID)
	case "DELIST":
		// 下架产品
		return s.delistProductViaSHEINAPI(ctx, prod, storeID)
	case "NONE":
		// 不执行任何操作
		s.logger.Debug("低利润率策略为NONE，不执行操作")
		return nil
	default:
		s.logger.Warnf("未知的低利润率策略: %s", action)
		return nil
	}
}

// handleProfitRateRestore 处理利润率恢复情况
func (s *inventorySyncServiceImpl) handleProfitRateRestore(
	ctx context.Context,
	prod *InventoryProductSnapshot,
	skuMapping *SKUMappingData,
	strategy *listingruntime.OperationStrategy,
	storeID int64,
	profitRate float64,
) error {
	// 检查当前库存是否为0，如果不是0则不需要恢复
	currentStock := skuMapping.Stock
	if currentStock > 0 {
		s.logger.WithFields(logrus.Fields{
			"current_stock": currentStock,
		}).Debug("当前库存不为0，无需恢复")
		return nil
	}

	// 获取恢复库存的数量配置
	restoreStock := s.getRestoreStockAmount(strategy)

	s.logger.WithFields(logrus.Fields{
		"product_id":    prod.ProductID,
		"spu_name":      prod.PlatformProductID,
		"sku":           s.getStringValue(skuMapping.MappingInfo.SKU),
		"asin":          skuMapping.MappingInfo.ProductID,
		"profit_rate":   profitRate,
		"min_profit":    strategy.MinProfitRate,
		"restore_stock": restoreStock,
	}).Info("利润率恢复正常，恢复产品库存")

	return s.updateProductStockViaSHEINAPI(ctx, prod, skuMapping, restoreStock, storeID)
}

// getRestoreStockAmount 获取恢复库存的数量
func (s *inventorySyncServiceImpl) getRestoreStockAmount(strategy *listingruntime.OperationStrategy) int {
	if strategy.RestoreStockAmount > 0 {
		return strategy.RestoreStockAmount
	}
	return 10 // 默认恢复10个库存
}
