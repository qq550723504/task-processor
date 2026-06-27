// package pricing 提供核价服务接口定义
package pricing

import (
	"context"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	temupricing "task-processor/internal/temu/api/pricing"
)

// StoreConfigProvider 店铺配置提供者接口
type StoreConfigProvider interface {
	IsRebargainEnabled() bool
	GetPriceType() string
	GetPriceRejectStrategy() string
}

// ProductDataProvider 产品数据提供者接口
type ProductDataProvider interface {
	GetProductImportMapping(skuSN string, storeID int64) (*listingadmin.ProductImportMappingRespDTO, error)
	GetProductImportMappingBySku(skuSN string, storeID int64) (*listingadmin.ProductImportMappingRespDTO, error)
	GetPricingRules(storeID int64) ([]listingadmin.PricingRuleRespDTO, error)
	CalculateOriginCostPriceWithAmazon(
		mapping *listingadmin.ProductImportMappingRespDTO,
		supplierPrice float64,
		amazonProduct *model.Product,
		useAmazonPrice bool,
		priceType string,
	) float64
}

// PriceCalculator 价格计算器接口
type PriceCalculator interface {
	GetDefaultPricingRules(originCostPrice float64, rules *[]listingadmin.PricingRuleRespDTO) *listingadmin.PricingRuleRespDTO
	CalculateMinAcceptablePrice(originCostPrice float64, rule *listingadmin.PricingRuleRespDTO) float64
}

// ProductFetcher 产品获取器接口
type ProductFetcher interface {
	FetchProductWithRetry(productID, region string, storeID int64, maxRetries int) (*model.Product, error)
}

// DecisionMaker 决策制定者接口
type DecisionMaker interface {
	MakeDecision(item *temupricing.Sku, storeID int64) (*temupricing.Decision, error)
	MakeDecisionForSalesBoost(goods *temupricing.SalesBoostGoods, sku *temupricing.SalesBoostSku, storeID int64) (*temupricing.Decision, error)
}

// PricingProcessor 核价处理器接口
type PricingProcessor interface {
	ProcessPendingPrices(ctx context.Context) (*temupricing.Statistics, error)
	ProcessPendingPricesWithAmazon(ctx context.Context, configProvider any) (*temupricing.Statistics, error)
}
