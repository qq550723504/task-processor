// Package pricing 提供核价服务接口定义
package pricing

import (
	"context"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/temu/api/models"
)

// StoreConfigProvider 店铺配置提供者接口
type StoreConfigProvider interface {
	IsRebargainEnabled() bool
	GetPriceType() string
	GetPriceRejectStrategy() string
}

// ProductDataProvider 产品数据提供者接口
type ProductDataProvider interface {
	GetProductImportMapping(skuSN string, storeID int64) (*api.ProductImportMappingRespDTO, error)
	GetProductImportMappingBySku(skuSN string, storeID int64) (*api.ProductImportMappingRespDTO, error)
	GetPricingRules(storeID int64) ([]api.PricingRuleRespDTO, error)
	CalculateOriginCostPriceWithAmazon(
		mapping *api.ProductImportMappingRespDTO,
		supplierPrice float64,
		amazonProduct *model.Product,
		useAmazonPrice bool,
		priceType string,
	) float64
}

// PriceCalculator 价格计算器接口
type PriceCalculator interface {
	GetDefaultPricingRules(originCostPrice float64, rules *[]api.PricingRuleRespDTO) *api.PricingRuleRespDTO
	CalculateMinAcceptablePrice(originCostPrice float64, rule *api.PricingRuleRespDTO) float64
}

// ProductFetcher 产品获取器接口
type ProductFetcher interface {
	FetchProductWithRetry(productID, region string, storeID int64, maxRetries int) (*model.Product, error)
}

// DecisionMaker 决策制定者接口
type DecisionMaker interface {
	MakeDecision(item *models.PricingSku, storeID int64) (*models.PricingDecision, error)
	MakeDecisionForSalesBoost(goods *models.SalesBoostGoods, sku *models.SalesBoostSku, storeID int64) (*models.PricingDecision, error)
}

// PricingProcessor 核价处理器接口
type PricingProcessor interface {
	ProcessPendingPrices(ctx context.Context) (*models.PricingStatistics, error)
	ProcessPendingPricesWithAmazon(ctx context.Context, configProvider any) (*models.PricingStatistics, error)
}
