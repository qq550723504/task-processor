// Package scheduler 提供TEMU平台调度器相关服务
package scheduler

import (
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	managementapi "task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// CostCalculationConfig 成本计算配置
type CostCalculationConfig struct {
	// 固定成本配置
	FixedCostAmount    float64 `json:"fixedCostAmount"`    // 固定成本金额
	FixedCostPercent   float64 `json:"fixedCostPercent"`   // 固定成本百分比
	ShippingCost       float64 `json:"shippingCost"`       // 运费成本
	ProcessingFee      float64 `json:"processingFee"`      // 处理费用
	PlatformCommission float64 `json:"platformCommission"` // 平台佣金百分比
}

// getProductCostPrice 获取产品成本价格（包含固定成本）
func (s *inventorySyncServiceImpl) getProductCostPrice(
	prod *managementapi.ProductDataDTO,
	skuMapping *TemuSkuInfo,
	storeID int64,
) float64 {
	mappingInfo := skuMapping.MappingInfo

	// 获取基础成本价格
	baseCostPrice := s.getFloatValue(mappingInfo.CostPrice)
	if baseCostPrice <= 0 {
		baseCostPrice = s.parsePrice(prod.OriginalPrice.String())
		if baseCostPrice <= 0 {
			baseCostPrice = s.parsePrice(prod.SpecialPrice.String())
		}
	}

	if baseCostPrice <= 0 {
		s.logger.WithFields(logrus.Fields{
			"product_id": prod.ProductID,
			"sku":        s.getStringValue(mappingInfo.Sku),
		}).Debug("无法获取基础成本价格")
		return 0
	}

	// 获取成本计算配置
	costConfig := s.getCostCalculationConfig(storeID)

	// 计算总成本
	totalCost := s.calculateTotalCost(baseCostPrice, costConfig)

	return totalCost
}

// getAmazonProductCostPrice 获取Amazon产品成本价格（包含固定成本）
func (s *inventorySyncServiceImpl) getAmazonProductCostPrice(
	amazonProduct *model.Product,
	priceType string,
	storeID int64,
) float64 {
	// 获取Amazon基础价格
	basePrice := product.GetProductPrice(amazonProduct, priceType)
	if basePrice <= 0 {
		return 0
	}

	// 获取成本计算配置
	costConfig := s.getCostCalculationConfig(storeID)

	// 计算总成本
	totalCost := s.calculateTotalCost(basePrice, costConfig)

	return totalCost
}

// calculateTotalCost 计算总成本
func (s *inventorySyncServiceImpl) calculateTotalCost(basePrice float64, config *CostCalculationConfig) float64 {
	totalCost := basePrice

	// 添加固定金额成本
	totalCost += config.FixedCostAmount

	// 添加固定百分比成本
	if config.FixedCostPercent > 0 {
		totalCost += basePrice * (config.FixedCostPercent / 100.0)
	}

	// 添加运费成本
	totalCost += config.ShippingCost

	// 添加处理费用
	totalCost += config.ProcessingFee

	// 添加平台佣金
	if config.PlatformCommission > 0 {
		totalCost += basePrice * (config.PlatformCommission / 100.0)
	}

	return totalCost
}

// getCostCalculationConfig 获取成本计算配置
func (s *inventorySyncServiceImpl) getCostCalculationConfig(storeID int64) *CostCalculationConfig {
	// 尝试从管理系统获取店铺级成本配置
	strategy, err := s.managementClient.GetOperationStrategyClient().GetOperationStrategyByStoreId(storeID)
	if err == nil && strategy != nil && strategy.IsEnabled() {
		// 如果运营策略中有成本配置，使用策略中的配置
		// 这里可以扩展 OperationStrategyDTO 来包含成本配置字段
		return &CostCalculationConfig{
			FixedCostAmount:    s.getFixedCostAmount(strategy),
			FixedCostPercent:   s.getFixedCostPercent(strategy),
			ShippingCost:       s.getShippingCost(strategy),
			ProcessingFee:      s.getProcessingFee(strategy),
			PlatformCommission: s.getPlatformCommission(strategy),
		}
	}

	// 使用默认配置
	return s.getDefaultCostConfig()
}

// getFixedCostAmount 获取固定成本金额
func (s *inventorySyncServiceImpl) getFixedCostAmount(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.FixedCostAmount
	return 0 // 默认固定成本$0
}

// getFixedCostPercent 获取固定成本百分比
func (s *inventorySyncServiceImpl) getFixedCostPercent(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.FixedCostPercent
	return 0 // 默认0%
}

// getShippingCost 获取运费成本
func (s *inventorySyncServiceImpl) getShippingCost(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.ShippingCost
	return 0 // 默认运费$0
}

// getProcessingFee 获取处理费用
func (s *inventorySyncServiceImpl) getProcessingFee(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.ProcessingFee
	return 0 // 默认处理费$0
}

// getPlatformCommission 获取平台佣金百分比
func (s *inventorySyncServiceImpl) getPlatformCommission(strategy *managementapi.OperationStrategyDTO) float64 {
	// 可以从策略中获取，这里使用默认值
	// 如果策略中有相关字段，可以使用 strategy.PlatformCommission
	return 0 // 默认平台佣金0%
}

// getDefaultCostConfig 获取默认成本配置
func (s *inventorySyncServiceImpl) getDefaultCostConfig() *CostCalculationConfig {
	return &CostCalculationConfig{
		FixedCostAmount:    0, // 固定成本$0
		FixedCostPercent:   0, // 固定成本0%
		ShippingCost:       0, // 运费$0
		ProcessingFee:      0, // 处理费$0
		PlatformCommission: 0, // 平台佣金0%
	}
}
