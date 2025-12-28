// Package shein 提供SHEIN平台的利润策略管理功能
package shein

import (
	"fmt"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/management/api"
	shops "task-processor/internal/common/shein"

	"github.com/sirupsen/logrus"
)

// ProfitStrategyManager 利润策略管理器
type ProfitStrategyManager struct {
	strategy     *api.OperationStrategyDTO
	apiClient    *shops.ShopAPIClient
	shelfManager *ShelfOperationManager
	priceUpdater *PriceUpdater
	stockUpdater *StockUpdater
}

// NewProfitStrategyManager 创建利润策略管理器
func NewProfitStrategyManager(strategy *api.OperationStrategyDTO, apiClient *shops.ShopAPIClient) *ProfitStrategyManager {
	return &ProfitStrategyManager{
		strategy:     strategy,
		apiClient:    apiClient,
		shelfManager: NewShelfOperationManager(apiClient),
		priceUpdater: NewPriceUpdater(strategy),
		stockUpdater: NewStockUpdater(strategy, apiClient),
	}
}

// ExecuteLowProfit 执行低利润率策略
func (m *ProfitStrategyManager) ExecuteLowProfit(
	prod *api.ProductDataDTO,
	skuMapping *SKUMappingData,
	amazonProduct *model.Product,
) error {
	if !m.strategy.IsEnabled() || m.strategy.LowProfitAction == "NONE" {
		return nil
	}

	// 从 CostPriceInfo 获取 SHEIN 销售价格(字符串类型需要转换)
	salePrice := parsePriceString(skuMapping.CostPriceInfo.CostPrice)

	// 从 SKUMappingData 的 AmazonMonitorData 获取成本价
	var costPrice float64
	if skuMapping.AmazonMonitorData != nil {
		costPrice = skuMapping.AmazonMonitorData.Price
	}

	if salePrice <= 0 || costPrice <= 0 {
		logrus.WithFields(logrus.Fields{
			"sku":        skuMapping.MappingInfo.SKU,
			"sale_price": salePrice,
			"cost_price": costPrice,
		}).Debug("销售价或成本价无效，跳过利润率检查")
		return nil
	}

	profitRate := (salePrice - costPrice) / costPrice * 100

	// 利润率达标，不需要执行策略
	if profitRate >= m.strategy.MinProfitRate {
		return nil
	}

	logrus.WithFields(logrus.Fields{
		"sku":         skuMapping.MappingInfo.SKU,
		"sale_price":  salePrice,
		"cost_price":  costPrice,
		"profit_rate": profitRate,
		"min_rate":    m.strategy.MinProfitRate,
		"action":      m.strategy.LowProfitAction,
	}).Info("触发低利润率策略")

	switch m.strategy.LowProfitAction {
	case "OFF_SHELF":
		return m.shelfManager.OffShelfProduct(prod)
	case "UPDATE_PRICE":
		newPrice := costPrice * (1 + m.strategy.MinProfitRate/100)
		return m.priceUpdater.UpdatePrice(prod, skuMapping, newPrice)
	case "SET_ZERO_STOCK":
		return m.stockUpdater.UpdateStock(prod, skuMapping, 0)
	}

	return nil
}

// parsePriceString 解析价格字符串为 float64
func parsePriceString(priceStr string) float64 {
	if priceStr == "" {
		return 0
	}

	var price float64
	// 尝试解析价格字符串
	if _, err := fmt.Sscanf(priceStr, "%f", &price); err != nil {
		return 0
	}

	return price
}
