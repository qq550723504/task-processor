// Package scheduler 提供TEMU平台库存监控策略处理逻辑
package syncsvc

import (
	"context"

	"task-processor/internal/domain/model"
	managementapi "task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// handlePriceChangeWithStrategy 根据价格变化策略处理（基于利润率）
func (s *inventorySyncServiceImpl) handlePriceChangeWithStrategy(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *TemuSkuInfo,
	operationStrategy *managementapi.OperationStrategyDTO, // 预获取的运营策略
) error {
	s.logger.WithFields(logrus.Fields{
		"product_id": prod.ProductID,
		"asin":       amazonProduct.Asin,
	}).Info("检测到TEMU产品价格变化，开始处理策略")

	// 使用成本计算函数获取旧成本价格（包含固定成本）
	oldCostPrice := s.getProductCostPrice(prod, skuMapping, prod.StoreID)
	if oldCostPrice <= 0 {
		s.logger.Debug("无法获取旧成本价格，跳过价格变化处理")
		return nil
	}

	// 获取店铺配置的价格类型
	priceType := s.getStorePriceType(prod.StoreID)

	// 使用成本计算函数获取Amazon新成本价格（包含固定成本）
	newCostPrice := s.getAmazonProductCostPrice(amazonProduct, priceType, prod.StoreID)
	if newCostPrice <= 0 {
		s.logger.Debug("无法获取Amazon新成本价格，跳过价格变化处理")
		return nil
	}

	// 获取TEMU产品的销售价格
	temuSalePrice := s.parsePrice(prod.OriginalPrice.String())
	if temuSalePrice <= 0 {
		temuSalePrice = s.parsePrice(prod.SpecialPrice.String())
	}

	if temuSalePrice <= 0 {
		s.logger.Debug("无法获取TEMU销售价格，跳过价格变化处理")
		return nil
	}

	// 计算当前利润率和新利润率
	oldProfitRate := (temuSalePrice - oldCostPrice) / temuSalePrice
	newProfitRate := (temuSalePrice - newCostPrice) / temuSalePrice

	// 获取最低利润率阈值
	minProfitRate := operationStrategy.MinProfitRate
	if minProfitRate <= 0 {
		s.logger.Error("利润率阈值为0")
		return nil
	}

	// 判断利润率变化类型并执行相应策略
	if newProfitRate < minProfitRate && oldProfitRate >= minProfitRate {
		// 利润率从正常降到低于阈值，执行低利润率策略
		return s.handleLowProfitRate(ctx, prod, skuMapping, operationStrategy, newProfitRate)
	} else if oldProfitRate < minProfitRate && newProfitRate >= minProfitRate {
		// 利润率从低于阈值恢复到正常，执行恢复策略
		return s.handleProfitRateRestore(ctx, prod, skuMapping)
	}

	return nil
}

// handleStockChangeWithStrategy 根据库存变化策略处理
func (s *inventorySyncServiceImpl) handleStockChangeWithStrategy(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *TemuSkuInfo,
	operationStrategy *managementapi.OperationStrategyDTO, // 预获取的运营策略
) error {
	currentStock := s.extractStockFromProduct(amazonProduct)

	s.logger.WithFields(logrus.Fields{
		"product_id":     prod.ProductID,
		"asin":           amazonProduct.Asin,
		"current_stock":  currentStock,
		"previous_stock": skuMapping.UsableInventory,
	}).Info("检测到TEMU产品库存变化，开始处理策略")

	oldStock := skuMapping.UsableInventory
	newStock := currentStock

	// 处理缺货情况
	if newStock == 0 && oldStock > 0 {
		return s.handleOutOfStock(ctx, prod, skuMapping, operationStrategy)
	}

	// 处理库存变化
	changeAmount := newStock - oldStock
	if s.absInt(changeAmount) >= operationStrategy.StockChangeThreshold {
		return s.handleStockChange(ctx, prod, skuMapping, operationStrategy, oldStock, newStock)
	}

	return nil
}

// handleLowProfitRate 处理低利润率情况
func (s *inventorySyncServiceImpl) handleLowProfitRate(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuSkuInfo,
	strategy *managementapi.OperationStrategyDTO,
	profitRate float64,
) error {
	action := strategy.LowProfitAction
	if action == "" {
		action = "NONE"
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":  prod.ProductID,
		"spu_name":    prod.PlatformProductID,
		"sku":         s.getStringValue(skuMapping.MappingInfo.Sku),
		"asin":        skuMapping.MappingInfo.ProductId,
		"profit_rate": profitRate,
		"min_profit":  strategy.MinProfitRate,
		"action":      action,
	}).Info("检测到低利润率，执行策略")

	switch action {
	case "SET_ZERO_STOCK":
		// 将库存设为0，避免亏损销售
		return s.updateProductStockViaTEMUAPI(ctx, prod, skuMapping, 0)
	case "DELIST":
		// 下架产品
		return s.delistProductViaTEMUAPI(ctx, prod, skuMapping)
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
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuSkuInfo,
) error {

	restoreStock := skuMapping.AmazonMonitorData.Stock

	return s.updateProductStockViaTEMUAPI(ctx, prod, skuMapping, restoreStock)
}

// handleOutOfStock 处理缺货情况
func (s *inventorySyncServiceImpl) handleOutOfStock(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuSkuInfo,
	strategy *managementapi.OperationStrategyDTO,
) error {
	action := strategy.OutOfStockAction
	if action == "" {
		action = "NONE"
	}

	switch action {
	case "DELIST":
		// 下架产品
		return s.delistProductViaTEMUAPI(ctx, prod, skuMapping)
	case "UPDATE_STOCK", "SET_ZERO_STOCK":
		// 更新库存为0
		return s.updateProductStockViaTEMUAPI(ctx, prod, skuMapping, 0)
	case "NONE":
		// 不执行任何操作
		s.logger.Debug("缺货策略为NONE，不执行操作")
		return nil
	default:
		s.logger.Warnf("未知的缺货策略: %s", action)
		return nil
	}
}

// handleStockChange 处理库存变化
func (s *inventorySyncServiceImpl) handleStockChange(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuSkuInfo,
	strategy *managementapi.OperationStrategyDTO,
	oldStock, newStock int,
) error {
	action := strategy.StockChangeAction
	if action == "" {
		action = "NONE"
	}

	switch action {
	case "UPDATE_STOCK":
		// 根据比例更新库存
		targetStock := s.calculateTargetStock(newStock, strategy.StockUpdateRatio)
		return s.updateProductStockViaTEMUAPI(ctx, prod, skuMapping, targetStock)
	case "NONE":
		// 不执行任何操作
		s.logger.Debug("库存变化策略为NONE，不执行操作")
		return nil
	default:
		s.logger.Warnf("未知的库存变化策略: %s", action)
		return nil
	}
}

// calculateTargetStock 计算目标库存
func (s *inventorySyncServiceImpl) calculateTargetStock(amazonStock int, ratio float64) int {
	if ratio <= 0 {
		ratio = 1.0 // 默认1:1
	}

	targetStock := int(float64(amazonStock) * ratio)
	if targetStock < 0 {
		targetStock = 0
	}

	return targetStock
}
