// Package service 提供核价服务相关的接口定义。
package service

import (
	"context"
	"task-processor/internal/common/pricing/model"
)

// PricingCalculator 价格计算器接口
type PricingCalculator interface {
	// CalculatePrice 根据规则计算价格
	CalculatePrice(ctx context.Context, originPrice float64, rules []model.PricingRule) (float64, *model.PricingRule, error)

	// FindApplicableRule 查找适用的规则
	FindApplicableRule(price float64, rules []model.PricingRule) *model.PricingRule
}

// CostPriceProvider 成本价提供者接口
type CostPriceProvider interface {
	// GetOriginCostPrice 获取原始成本价
	GetOriginCostPrice(ctx context.Context, productID string, storeID int64) (float64, error)

	// GetImportMapping 获取商品导入映射
	GetImportMapping(ctx context.Context, productID string, storeID int64) (*model.ImportMapping, error)
}

// PricingDecisionMaker 核价决策制定者接口
type PricingDecisionMaker interface {
	// MakeDecision 制定核价决策
	MakeDecision(ctx context.Context, pricingCtx *model.PricingContext) (*model.PricingResult, error)

	// ValidateContext 验证核价上下文
	ValidateContext(pricingCtx *model.PricingContext) error
}

// PlatformAdapter 平台适配器接口
type PlatformAdapter interface {
	// GetPlatformName 获取平台名称
	GetPlatformName() string

	// PreprocessContext 预处理核价上下文
	PreprocessContext(ctx context.Context, pricingCtx *model.PricingContext) error

	// PostprocessResult 后处理核价结果
	PostprocessResult(ctx context.Context, result *model.PricingResult) error

	// ExecuteDecision 执行核价决策
	ExecuteDecision(ctx context.Context, result *model.PricingResult) error
}

// CommonPricingService 通用核价服务接口
type CommonPricingService interface {
	// ProcessSingleProduct 处理单个商品核价
	ProcessSingleProduct(ctx context.Context, pricingCtx *model.PricingContext) (*model.PricingResult, error)

	// ProcessBatchProducts 批量处理商品核价
	ProcessBatchProducts(ctx context.Context, contexts []*model.PricingContext) (*model.BatchPricingResult, error)

	// RegisterAdapter 注册平台适配器
	RegisterAdapter(platformName string, adapter PlatformAdapter)

	// GetAdapter 获取平台适配器
	GetAdapter(platformName string) (PlatformAdapter, error)
}
