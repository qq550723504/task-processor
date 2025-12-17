// Package core 提供核价引擎的核心实现，保持原有逻辑不变。
package core

import (
	"fmt"
	"task-processor/internal/common/management"
	"task-processor/internal/common/management/api"

	"github.com/sirupsen/logrus"
)

// PricingEngine 核价引擎，整合TEMU和SHEIN的原有逻辑
type PricingEngine struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewPricingEngine 创建核价引擎
func NewPricingEngine(managementClient *management.ClientManager) *PricingEngine {
	return &PricingEngine{
		managementClient: managementClient,
		logger:           logrus.WithField("component", "PricingEngine"),
	}
}

// ApplyPricingRule 应用核价规则 (来自SHEIN的原有逻辑)
func (e *PricingEngine) ApplyPricingRule(originPrice float64, rule api.PricingRuleRespDTO) float64 {
	if rule.RuleValue == nil {
		return originPrice
	}

	switch rule.RuleType {
	case "fixed":
		// 固定加价：成本价 + 固定值
		return originPrice + *rule.RuleValue
	case "percent":
		// 加价百分比：成本价 × (1 + 百分比)
		return originPrice * (1 + *rule.RuleValue)
	case "percent_plus_fixed":
		// 百分比加固定值：成本价 × (1 + 百分比) + 固定值
		percentPrice := originPrice * (1 + *rule.RuleValue)
		if rule.FixedValue != nil {
			return percentPrice + *rule.FixedValue
		}
		return percentPrice
	case "multiple":
		// 倍数：倍数 × 成本价
		return *rule.RuleValue * originPrice
	case "discount":
		// 折扣率：成本价 × (1 - 折扣率)
		return originPrice * (1 - *rule.RuleValue)
	case "fixed_price":
		// 固定价格
		return *rule.RuleValue
	}
	return originPrice
}

// GetPricingRule 获取核价规则 (来自TEMU的原有逻辑)
func (e *PricingEngine) GetPricingRule(tenantID, storeID int64) (*api.PricingRuleRespDTO, error) {
	pricingRuleClient := e.managementClient.GetPricingRuleClient()
	if pricingRuleClient == nil {
		return nil, fmt.Errorf("核价规则客户端未初始化")
	}

	req := &api.PricingRuleReqDTO{
		TenantID: tenantID,
		StoreID:  &storeID,
	}

	pricingRule, err := pricingRuleClient.GetPricingRule(req)
	if err != nil {
		return nil, fmt.Errorf("获取核价规则失败: %w", err)
	}

	// 验证核价规则是否启用
	if pricingRule.Status != 0 {
		return nil, fmt.Errorf("核价规则未启用: %s", pricingRule.Name)
	}

	return pricingRule, nil
}

// GetProductImportMapping 获取产品导入映射 (来自TEMU的原有逻辑)
func (e *PricingEngine) GetProductImportMapping(productID string, storeID int64) (*api.ProductImportMappingRespDTO, error) {
	mappingClient := e.managementClient.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

	// 使用SKU SN作为平台产品ID查询
	req := &api.ProductImportMappingGetBySkuReqDTO{
		Sku:     productID,
		StoreId: storeID,
	}

	mapping, err := mappingClient.GetProductImportMappingBySku(req)
	if err != nil {
		return nil, fmt.Errorf("获取产品导入映射失败: %w", err)
	}

	return mapping, nil
}

// GetProductImportMappingByPlatformID 通过平台产品ID获取映射 (来自SHEIN的原有逻辑)
func (e *PricingEngine) GetProductImportMappingByPlatformID(platformProductID string) (*api.ProductImportMappingRespDTO, error) {
	mappingClient := e.managementClient.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

	req := &api.ProductImportMappingGetReqDTO{
		PlatformProductId: platformProductID,
	}

	mapping, err := mappingClient.GetProductImportMappingByPlatformProductId(req)
	if err != nil {
		return nil, fmt.Errorf("获取产品导入映射失败: %w", err)
	}

	return mapping, nil
}

// CalculateOriginCostPrice 计算原始成本价 (来自TEMU的原有逻辑)
func (e *PricingEngine) CalculateOriginCostPrice(mapping *api.ProductImportMappingRespDTO, currentPrice float64) float64 {
	if mapping == nil {
		return 0
	}

	// 优先使用直接的成本价
	if mapping.CostPrice != nil && *mapping.CostPrice > 0 {
		return *mapping.CostPrice
	}

	// 如果没有成本价，使用售价倍数反推
	if mapping.SalePriceMultiplier != nil && *mapping.SalePriceMultiplier > 0 {
		return currentPrice / *mapping.SalePriceMultiplier
	}

	return 0
}

// GetAutoPrice 获取自动核价 (来自SHEIN的原有逻辑)
func (e *PricingEngine) GetAutoPrice(originPrice float64, rules []api.PricingRuleRespDTO) float64 {
	for _, rule := range rules {
		// 判断是否在规则范围内
		if rule.PriceMin != nil && rule.PriceMax != nil &&
			originPrice >= *rule.PriceMin && originPrice < *rule.PriceMax {
			return e.ApplyPricingRule(originPrice, rule)
		}
	}
	return originPrice // 没有匹配规则则原价
}
