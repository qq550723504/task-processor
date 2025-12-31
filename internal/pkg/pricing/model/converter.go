// Package model 提供模型转换相关的工具方法。
package model

import "task-processor/internal/pkg/management/api"

// ConvertPricingRulesFromAPI 从管理端API模型转换为统一的核价规则模型
func ConvertPricingRulesFromAPI(apiRules []api.PricingRuleRespDTO) []PricingRule {
	if len(apiRules) == 0 {
		return nil
	}

	rules := make([]PricingRule, 0, len(apiRules))
	for _, apiRule := range apiRules {
		rule := PricingRule{}
		rule.FromManagementAPI(&apiRule)
		rules = append(rules, rule)
	}

	return rules
}

// ConvertPricingRuleFromAPI 从管理端API模型转换为统一的核价规则模型
func ConvertPricingRuleFromAPI(apiRule *api.PricingRuleRespDTO) *PricingRule {
	if apiRule == nil {
		return nil
	}

	rule := &PricingRule{}
	rule.FromManagementAPI(apiRule)
	return rule
}

// ConvertStoreConfigFromAPI 从管理端API模型转换店铺配置
func ConvertStoreConfigFromAPI(apiStore *api.StoreRespDTO) *StoreConfig {
	if apiStore == nil {
		return nil
	}

	return &StoreConfig{
		ID:                      apiStore.ID,
		Name:                    apiStore.Name,
		EnableAutoPrice:         apiStore.EnableAutoPrice,
		EnableRebargain:         apiStore.EnableRebargain,
		TemuPriceRejectStrategy: apiStore.TemuPriceRejectStrategy,
	}
}

// StoreConfig 店铺配置统一模型
type StoreConfig struct {
	ID                       int64  `json:"id"`
	Name                     string `json:"name"`
	EnableAutoPrice          *bool  `json:"enable_auto_price,omitempty"`
	EnableRebargain          *bool  `json:"enable_rebargain,omitempty"`
	TemuPriceRejectStrategy  string `json:"temu_price_reject_strategy,omitempty"`
	SheinPriceRejectStrategy string `json:"shein_price_reject_strategy,omitempty"`
}

// ImportMapping 商品导入映射统一模型
type ImportMapping struct {
	CostPrice           *float64 `json:"cost_price"`
	SalePriceMultiplier *float64 `json:"sale_price_multiplier"`
}

// ConvertImportMappingFromAPI 从管理端API模型转换导入映射
func ConvertImportMappingFromAPI(apiMapping *api.ProductImportMappingRespDTO) *ImportMapping {
	if apiMapping == nil {
		return nil
	}

	return &ImportMapping{
		CostPrice:           apiMapping.CostPrice,
		SalePriceMultiplier: apiMapping.SalePriceMultiplier,
	}
}
