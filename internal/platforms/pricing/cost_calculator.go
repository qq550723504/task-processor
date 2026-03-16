// Package pricing 提供通用的成本计算服务
package pricing

import (
	"task-processor/internal/core/config"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// CostCalculator 通用成本计算器
type CostCalculator struct {
	logger           *logrus.Entry
	managementClient *management.ClientManager
	enableDetailLog  bool // 是否启用详细日志
}

// NewCostCalculator 创建成本计算器
func NewCostCalculator(
	managementClient *management.ClientManager,
	logger *logrus.Entry,
	enableDetailLog bool,
) *CostCalculator {
	return &CostCalculator{
		logger:           logger,
		managementClient: managementClient,
		enableDetailLog:  enableDetailLog,
	}
}

// CalculateProductCost 计算产品成本价格
// baseCostPrice: 基础成本价格
// storeID: 店铺ID
// productID: 产品ID（用于日志）
// sku: SKU（用于日志）
func (c *CostCalculator) CalculateProductCost(
	baseCostPrice float64,
	storeID int64,
	productID string,
	sku string,
) float64 {
	if baseCostPrice <= 0 {
		c.logger.WithFields(logrus.Fields{
			"product_id": productID,
			"sku":        sku,
		}).Debug("基础成本价格无效")
		return 0
	}

	// 获取成本配置
	costConfig := c.getCostConfig(storeID)

	// 计算总成本
	totalCost := costConfig.CalculateTotalCost(baseCostPrice)

	// 如果启用详细日志，记录计算详情
	if c.enableDetailLog {
		c.logger.WithFields(logrus.Fields{
			"product_id":     productID,
			"sku":            sku,
			"base_cost":      baseCostPrice,
			"fixed_amount":   costConfig.FixedCostAmount,
			"fixed_percent":  costConfig.FixedCostPercent,
			"shipping_cost":  costConfig.ShippingCost,
			"processing_fee": costConfig.ProcessingFee,
			"platform_comm":  costConfig.PlatformCommission,
			"total_cost":     totalCost,
		}).Debug("成本价格计算完成")
	}

	return totalCost
}

// CalculateAmazonProductCost 计算Amazon产品成本价格
func (c *CostCalculator) CalculateAmazonProductCost(
	amazonProduct *model.Product,
	priceType string,
	storeID int64,
) float64 {
	// 获取Amazon基础价格
	basePrice := product.GetProductPrice(amazonProduct, priceType)
	if basePrice <= 0 {
		return 0
	}

	// 获取成本配置
	costConfig := c.getCostConfig(storeID)

	// 计算总成本
	totalCost := costConfig.CalculateTotalCost(basePrice)

	// 如果启用详细日志，记录计算详情
	if c.enableDetailLog {
		c.logger.WithFields(logrus.Fields{
			"asin":           amazonProduct.Asin,
			"price_type":     priceType,
			"base_price":     basePrice,
			"fixed_amount":   costConfig.FixedCostAmount,
			"fixed_percent":  costConfig.FixedCostPercent,
			"shipping_cost":  costConfig.ShippingCost,
			"processing_fee": costConfig.ProcessingFee,
			"platform_comm":  costConfig.PlatformCommission,
			"total_cost":     totalCost,
		}).Debug("Amazon产品成本价格计算完成")
	}

	return totalCost
}

// getCostConfig 获取成本配置
func (c *CostCalculator) getCostConfig(storeID int64) *config.CostCalculationConfig {
	// 如果没有管理客户端，直接使用默认配置
	if c.managementClient == nil {
		return config.DefaultCostCalculationConfig()
	}

	// 尝试从管理系统获取店铺级成本配置
	strategy, err := c.managementClient.GetOperationStrategyClient().GetOperationStrategyByStoreId(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		// 如果运营策略中有成本配置，使用策略中的配置
		// 这里可以扩展 OperationStrategyDTO 来包含成本配置字段
		return &config.CostCalculationConfig{
			FixedCostAmount:    c.getFixedCostAmount(strategy),
			FixedCostPercent:   c.getFixedCostPercent(strategy),
			ShippingCost:       c.getShippingCost(strategy),
			ProcessingFee:      c.getProcessingFee(strategy),
			PlatformCommission: c.getPlatformCommission(strategy),
		}
	}

	// 使用默认配置
	return config.DefaultCostCalculationConfig()
}

// getFixedCostAmount 获取固定成本金额
func (c *CostCalculator) getFixedCostAmount(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.FixedCostAmount
	return 0 // 默认固定成本$0
}

// getFixedCostPercent 获取固定成本百分比
func (c *CostCalculator) getFixedCostPercent(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.FixedCostPercent
	return 0 // 默认0%
}

// getShippingCost 获取运费成本
func (c *CostCalculator) getShippingCost(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.ShippingCost
	return 0 // 默认运费$0
}

// getProcessingFee 获取处理费用
func (c *CostCalculator) getProcessingFee(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.ProcessingFee
	return 0 // 默认处理费$0
}

// getPlatformCommission 获取平台佣金百分比
func (c *CostCalculator) getPlatformCommission(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.PlatformCommission
	return 0 // 默认平台佣金0%
}
