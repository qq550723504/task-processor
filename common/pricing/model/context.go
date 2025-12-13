// Package model 提供核价相关的数据模型定义。
package model

import (
	"context"
)

// PricingContext 核价上下文，包含核价所需的所有信息
type PricingContext struct {
	// 基础信息
	Ctx      context.Context
	TenantID int64
	StoreID  int64

	// 商品信息
	ProductID    string  // 平台商品ID
	SkuID        string  // SKU ID
	ProductName  string  // 商品名称
	CurrentPrice float64 // 当前价格
	SuggestPrice float64 // 建议价格

	// 店铺配置
	StoreConfig *StoreConfig

	// 核价规则
	PricingRules []PricingRule

	// 成本信息
	OriginCostPrice float64 // 原始成本价
	ImportMapping   *ImportMapping

	// 平台特定数据
	PlatformData interface{} // 平台特定的额外数据
}

// Validate 验证核价上下文的有效性
func (pc *PricingContext) Validate() error {
	if pc.TenantID <= 0 {
		return ErrInvalidTenantID
	}
	if pc.StoreID <= 0 {
		return ErrInvalidStoreID
	}
	if pc.ProductID == "" {
		return ErrInvalidProductID
	}
	if pc.CurrentPrice < 0 {
		return ErrInvalidPrice
	}
	return nil
}

// HasValidCostPrice 检查是否有有效的成本价
func (pc *PricingContext) HasValidCostPrice() bool {
	return pc.OriginCostPrice > 0
}

// IsAutoPricingEnabled 检查是否启用自动核价
func (pc *PricingContext) IsAutoPricingEnabled() bool {
	if pc.StoreConfig == nil || pc.StoreConfig.EnableAutoPrice == nil {
		return false
	}
	return *pc.StoreConfig.EnableAutoPrice
}

// IsRebargainEnabled 检查是否启用重新议价（TEMU特有）
func (pc *PricingContext) IsRebargainEnabled() bool {
	if pc.StoreConfig == nil || pc.StoreConfig.EnableRebargain == nil {
		return false
	}
	return *pc.StoreConfig.EnableRebargain
}

// GetPriceRejectStrategy 获取价格拒绝策略
func (pc *PricingContext) GetPriceRejectStrategy() string {
	if pc.StoreConfig == nil {
		return "KEEP_ONLINE" // 默认保留在售
	}

	// TEMU平台的拒绝策略
	if pc.StoreConfig.TemuPriceRejectStrategy != "" {
		return pc.StoreConfig.TemuPriceRejectStrategy
	}

	// SHEIN平台的拒绝策略
	if pc.StoreConfig.SheinPriceRejectStrategy != "" {
		return pc.StoreConfig.SheinPriceRejectStrategy
	}

	return "KEEP_ONLINE"
}
