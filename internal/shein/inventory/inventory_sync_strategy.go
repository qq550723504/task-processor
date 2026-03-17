// Package operation 提供SHEIN平台调度器相关服务
package inventory

import (
	"context"

	"task-processor/internal/domain/model"
	managementapi "task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// handleStockChangeWithStrategy 根据营销策略处理库存变化
func (s *inventorySyncServiceImpl) handleStockChangeWithStrategy(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	amazonProduct *model.Product,
	skuMapping *SKUMappingData,
	storeID int64,
) error {
	// 获取运营策略
	strategyClient := s.managementClient.GetOperationStrategyClient()
	strategy, err := strategyClient.GetOperationStrategyByStoreId(storeID)
	if err != nil {
		s.logger.WithError(err).Debug("获取运营策略失败，跳过自动处理")
		return nil
	}

	// 检查策略是否启用
	if strategy == nil || !strategy.IsEnabled() {
		s.logger.Debug("运营策略未启用，跳过自动处理")
		return nil
	}

	oldStock := skuMapping.Stock
	newStock := s.extractStockFromProduct(amazonProduct)

	// 处理缺货情况
	if newStock == 0 && oldStock > 0 {
		return s.handleOutOfStock(ctx, prod, skuMapping, strategy, storeID)
	}

	// 处理库存变化
	changeAmount := newStock - oldStock
	if s.absInt(changeAmount) >= strategy.StockChangeThreshold {
		return s.handleStockChange(ctx, prod, skuMapping, strategy, storeID, oldStock, newStock)
	}

	return nil
}

// handleOutOfStock 处理缺货情况
func (s *inventorySyncServiceImpl) handleOutOfStock(
	ctx context.Context,
	prod *managementapi.ProductDataDTO,
	skuMapping *SKUMappingData,
	strategy *managementapi.OperationStrategyDTO,
	storeID int64,
) error {
	action := strategy.OutOfStockAction
	if action == "" {
		action = "NONE"
	}

	s.logger.WithFields(logrus.Fields{
		"product_id": prod.ProductID,
		"spu_name":   prod.PlatformProductID,
		"sku":        s.getStringValue(skuMapping.MappingInfo.Sku),
		"asin":       skuMapping.MappingInfo.ProductId,
		"action":     action,
	}).Info("检测到Amazon缺货，执行策略")

	switch action {
	case "DELIST":
		// 下架产品
		return s.delistProductViaSHEINAPI(ctx, prod, storeID)
	case "UPDATE_STOCK", "SET_ZERO_STOCK":
		// 更新库存为0
		return s.updateProductStockViaSHEINAPI(ctx, prod, skuMapping, 0, storeID)
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
	skuMapping *SKUMappingData,
	strategy *managementapi.OperationStrategyDTO,
	storeID int64,
	oldStock, newStock int,
) error {
	action := strategy.StockChangeAction
	if action == "" {
		action = "NONE"
	}

	s.logger.WithFields(logrus.Fields{
		"product_id":   prod.ProductID,
		"spu_name":     prod.PlatformProductID,
		"sku":          s.getStringValue(skuMapping.MappingInfo.Sku),
		"asin":         skuMapping.MappingInfo.ProductId,
		"old_stock":    oldStock,
		"new_stock":    newStock,
		"action":       action,
		"update_ratio": strategy.StockUpdateRatio,
	}).Info("检测到库存变化，执行策略")

	switch action {
	case "UPDATE_STOCK":
		// 根据比例更新库存
		targetStock := s.calculateTargetStock(newStock, strategy.StockUpdateRatio)
		return s.updateProductStockViaSHEINAPI(ctx, prod, skuMapping, targetStock, storeID)
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
