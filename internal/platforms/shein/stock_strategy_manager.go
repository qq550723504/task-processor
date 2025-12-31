// Package shein 提供SHEIN平台的库存策略管理功能
package shein

import (
	"math"
	"task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// StockStrategyManager 库存策略管理器
type StockStrategyManager struct {
	strategy     *api.OperationStrategyDTO
	apiClient    *shops.ShopAPIClient
	shelfManager *ShelfOperationManager
	stockUpdater *StockUpdater
}

// NewStockStrategyManager 创建库存策略管理器
func NewStockStrategyManager(strategy *api.OperationStrategyDTO, apiClient *shops.ShopAPIClient) *StockStrategyManager {
	return &StockStrategyManager{
		strategy:     strategy,
		apiClient:    apiClient,
		shelfManager: NewShelfOperationManager(apiClient),
		stockUpdater: NewStockUpdater(strategy, apiClient),
	}
}

// ExecuteStockChange 执行库存变化策略
func (m *StockStrategyManager) ExecuteStockChange(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
) error {
	if !m.strategy.IsEnabled() || m.strategy.StockChangeAction == "NONE" {
		return nil
	}

	oldStock := skuMapping.Stock
	newStock := extractStockFromProduct(amazonProduct)

	// 如果库存充足（31 表示 In Stock），则不做任何动作
	if newStock == 31 {
		return nil
	}

	changeAmount := int(math.Abs(float64(newStock - oldStock)))

	// 库存变化未达到阈值
	if changeAmount < m.strategy.StockChangeThreshold {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"sku":           skuMapping.MappingInfo.SKU,
		"old_stock":     oldStock,
		"new_stock":     newStock,
		"change_amount": changeAmount,
		"action":        m.strategy.StockChangeAction,
	}).Info("触发库存变化策略")

	switch m.strategy.StockChangeAction {
	case "OFF_SHELF":
		return m.shelfManager.OffShelfProduct(prod)
	case "UPDATE_STOCK":
		return m.stockUpdater.UpdateStock(prod, skuMapping, newStock)
	}

	return nil
}

// ExecuteOutOfStock 执行缺货策略
func (m *StockStrategyManager) ExecuteOutOfStock(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
) error {
	if !m.strategy.IsEnabled() || m.strategy.OutOfStockAction == "NONE" {
		return nil
	}

	// 产品有货，不需要执行缺货策略
	if amazonProduct.IsAvailable {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"sku":    skuMapping.MappingInfo.SKU,
		"action": m.strategy.OutOfStockAction,
	}).Info("触发缺货策略")

	switch m.strategy.OutOfStockAction {
	case "OFF_SHELF":
		return m.shelfManager.OffShelfProduct(prod)
	case "SET_ZERO_STOCK":
		return m.stockUpdater.UpdateStock(prod, skuMapping, 0)
	}

	return nil
}

// 注意：extractStockFromProduct 函数已在 amazon_fetcher.go 中定义
