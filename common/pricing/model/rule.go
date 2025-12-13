// Package model 提供核价规则相关的数据模型。
package model

import (
	"fmt"
	"task-processor/common/management/api"
)

// RuleType 规则类型
type RuleType string

const (
	RuleTypeFixed            RuleType = "fixed"              // 固定加价
	RuleTypePercent          RuleType = "percent"            // 加价百分比
	RuleTypePercentPlusFixed RuleType = "percent_plus_fixed" // 百分比加固定值
	RuleTypeMultiple         RuleType = "multiple"           // 倍数
	RuleTypeDiscount         RuleType = "discount"           // 折扣率
	RuleTypeFixedPrice       RuleType = "fixed_price"        // 固定价格
)

// PricingRule 统一的核价规则模型
type PricingRule struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	RuleCode    string `json:"rule_code"`
	Description string `json:"description"`
	StoreID     *int64 `json:"store_id,omitempty"`
	CategoryID  *int64 `json:"category_id,omitempty"`

	// 价格范围
	PriceMin *float64 `json:"price_min,omitempty"`
	PriceMax *float64 `json:"price_max,omitempty"`

	// 规则配置
	RuleType   RuleType `json:"rule_type"`
	RuleValue  *float64 `json:"rule_value,omitempty"`
	FixedValue *float64 `json:"fixed_value,omitempty"`

	// 条件配置
	AcceptCondition string `json:"accept_condition,omitempty"`
	RejectCondition string `json:"reject_condition,omitempty"`

	// 状态
	Status int `json:"status"` // 0-启用, 1-禁用
}

// FromManagementAPI 从管理端API模型转换
func (pr *PricingRule) FromManagementAPI(apiRule *api.PricingRuleRespDTO) {
	if apiRule == nil {
		return
	}

	pr.ID = apiRule.ID
	pr.Name = apiRule.Name
	pr.RuleCode = apiRule.RuleCode
	pr.Description = apiRule.Description
	pr.StoreID = apiRule.StoreID
	pr.CategoryID = apiRule.CategoryID
	pr.PriceMin = apiRule.PriceMin
	pr.PriceMax = apiRule.PriceMax
	pr.RuleType = RuleType(apiRule.RuleType)
	pr.RuleValue = apiRule.RuleValue
	pr.FixedValue = apiRule.FixedValue
	pr.AcceptCondition = apiRule.AcceptCondition
	pr.RejectCondition = apiRule.RejectCondition
	pr.Status = int(apiRule.Status)
}

// IsEnabled 检查规则是否启用
func (pr *PricingRule) IsEnabled() bool {
	return pr.Status == 0
}

// IsApplicable 检查规则是否适用于指定价格
func (pr *PricingRule) IsApplicable(price float64) bool {
	if !pr.IsEnabled() {
		return false
	}

	// 检查价格范围
	if pr.PriceMin != nil && price < *pr.PriceMin {
		return false
	}
	if pr.PriceMax != nil && price >= *pr.PriceMax {
		return false
	}

	return true
}

// ApplyRule 应用规则计算价格
func (pr *PricingRule) ApplyRule(originPrice float64) (float64, error) {
	if pr.RuleValue == nil {
		return originPrice, fmt.Errorf("规则值不能为空")
	}

	switch pr.RuleType {
	case RuleTypeFixed:
		// 固定加价：成本价 + 固定值
		return originPrice + *pr.RuleValue, nil

	case RuleTypePercent:
		// 加价百分比：成本价 × (1 + 百分比)
		return originPrice * (1 + *pr.RuleValue), nil

	case RuleTypePercentPlusFixed:
		// 百分比加固定值：成本价 × (1 + 百分比) + 固定值
		percentPrice := originPrice * (1 + *pr.RuleValue)
		if pr.FixedValue != nil {
			return percentPrice + *pr.FixedValue, nil
		}
		return percentPrice, nil

	case RuleTypeMultiple:
		// 倍数：倍数 × 成本价
		return *pr.RuleValue * originPrice, nil

	case RuleTypeDiscount:
		// 折扣率：成本价 × (1 - 折扣率)
		return originPrice * (1 - *pr.RuleValue), nil

	case RuleTypeFixedPrice:
		// 固定价格
		return *pr.RuleValue, nil

	default:
		return originPrice, fmt.Errorf("不支持的规则类型: %s", pr.RuleType)
	}
}

// Validate 验证规则配置的有效性
func (pr *PricingRule) Validate() error {
	if pr.Name == "" {
		return fmt.Errorf("规则名称不能为空")
	}

	if pr.RuleType == "" {
		return fmt.Errorf("规则类型不能为空")
	}

	// 验证规则值
	switch pr.RuleType {
	case RuleTypeFixed, RuleTypePercent, RuleTypeMultiple, RuleTypeDiscount, RuleTypeFixedPrice:
		if pr.RuleValue == nil {
			return fmt.Errorf("规则类型 %s 需要规则值", pr.RuleType)
		}
	case RuleTypePercentPlusFixed:
		if pr.RuleValue == nil {
			return fmt.Errorf("规则类型 %s 需要百分比值", pr.RuleType)
		}
		// FixedValue 可以为空，表示只使用百分比
	}

	// 验证价格范围
	if pr.PriceMin != nil && pr.PriceMax != nil && *pr.PriceMin >= *pr.PriceMax {
		return fmt.Errorf("最低价格不能大于等于最高价格")
	}

	return nil
}
