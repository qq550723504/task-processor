// package pricing 提供TEMU平台核价决策服务功能
package pricing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/core/config"
	appfetcher "task-processor/internal/crawler/fetcher"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"
	temupricing "task-processor/internal/temu/api/pricing"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// ServiceConfig 服务配置
type ServiceConfig struct {
	TenantID       int64
	StoreID        int64
	UseAmazonPrice bool
	MaxRetries     int
	CacheTimeout   time.Duration
}

// PricingDecisionService 核价决策服务
type PricingDecisionService struct {
	config         *ServiceConfig
	storeConfig    StoreConfigProvider
	dataService    ProductDataProvider
	ruleCalculator PriceCalculator
	productFetcher appfetcher.ProductFetcher
	logger         *logrus.Entry

	// 缓存相关
	cache      sync.Map
	cacheMutex sync.RWMutex
}

// CacheKey 缓存键
type CacheKey struct {
	Type      string
	ProductID string
	StoreID   int64
}

// CacheItem 缓存项
type CacheItem struct {
	Data      any
	ExpiresAt time.Time
}

// PricingContext 定价上下文
type PricingContext struct {
	ProductID          string
	SkuSN              string
	GoodsName          string
	SupplierPrice      float64
	StoreID            int64
	Mapping            *listingadmin.ProductImportMappingRespDTO
	AmazonProduct      *model.Product
	OriginCostPrice    float64
	PricingRule        *listingadmin.PricingRuleRespDTO
	MinAcceptablePrice float64
}

// NewPricingDecisionService 创建核价决策服务
func NewPricingDecisionService(
	runtime runtime,
	storeID int64,
	productFetcher appfetcher.ProductFetcher,
	platformConfig *config.PlatformConfig,
) (DecisionMaker, error) {
	config := &ServiceConfig{
		StoreID:        storeID,
		UseAmazonPrice: true,
		MaxRetries:     3,
		CacheTimeout:   5 * time.Minute,
	}

	// 从配置获取useAmazonPrice
	if platformConfig != nil {
		config.UseAmazonPrice = platformConfig.AutoPricing.UseAmazonPrice
	}

	return newPricingDecisionServiceWithConfig(runtime, config, productFetcher)
}

// newPricingDecisionServiceWithConfig 内部构造函数
func newPricingDecisionServiceWithConfig(
	runtime runtime,
	config *ServiceConfig,
	productFetcher appfetcher.ProductFetcher,
) (DecisionMaker, error) {
	if runtime == nil {
		return nil, errors.New("pricing runtime cannot be nil")
	}
	if config == nil {
		return nil, errors.New("config不能为空")
	}

	logger := logger.GetGlobalLogger("PricingDecisionService").WithField("storeID", config.StoreID)

	// 创建依赖服务
	storeConfig, err := NewStoreConfigService(config.StoreID, runtime)
	if err != nil {
		return nil, fmt.Errorf("创建店铺配置服务失败: %w", err)
	}

	dataService := NewPricingDataService(runtime, logger)
	ruleCalculator := NewPricingRuleCalculator(logger)

	return &PricingDecisionService{
		config:         config,
		storeConfig:    storeConfig,
		dataService:    dataService,
		ruleCalculator: ruleCalculator,
		productFetcher: productFetcher,
		logger:         logger,
	}, nil
}

// MakeDecision 对单个商品做出核价决策
func (s *PricingDecisionService) MakeDecision(item *temupricing.Sku, storeID int64) (*temupricing.Decision, error) {
	ctx := context.Background()

	decision := &temupricing.Decision{
		Sku: item,
	}

	// 参数校验
	if item == nil {
		decision.Action = temupricing.DecisionSkip
		decision.Reason = "商品信息为空"
		return decision, nil
	}

	// 使用配置中的storeID，如果传入的storeID不为0则使用传入的
	targetStoreID := s.config.StoreID
	if storeID != 0 {
		targetStoreID = storeID
	}

	// 构建定价上下文 - 这里需要tenantID，我们从配置或其他地方获取
	// 由于接口限制，我们需要从其他地方获取tenantID
	tenantID := int64(1) // 默认租户ID，实际应该从配置或上下文获取

	pricingCtx, err := s.buildPricingContext(ctx, item.SkuSN, item.GoodsName, item.SupplierPrice, tenantID, targetStoreID)
	if err != nil {
		decision.Action = temupricing.DecisionSkip
		decision.Reason = err.Error()
		s.logger.WithError(err).Warnf("构建定价上下文失败: %s", item.GoodsName)
		return decision, nil
	}

	// 记录定价信息
	s.logPricingInfo(pricingCtx)

	// 执行决策逻辑
	return s.makeDecisionByPrice(pricingCtx.SupplierPrice, pricingCtx.MinAcceptablePrice), nil
}

// buildPricingContext 构建定价上下文
func (s *PricingDecisionService) buildPricingContext(ctx context.Context, skuSN, goodsName string, supplierPrice float64, _ int64, storeID int64) (*PricingContext, error) {
	pricingCtx := &PricingContext{
		SkuSN:         skuSN,
		GoodsName:     goodsName,
		SupplierPrice: supplierPrice,
		StoreID:       storeID,
	}

	// 获取上架记录映射
	mapping, err := s.dataService.GetProductImportMapping(skuSN, storeID)
	if err != nil {
		return nil, fmt.Errorf("获取上架记录失败: %w", err)
	}
	pricingCtx.Mapping = mapping

	// 获取Amazon产品数据
	if mapping != nil && mapping.ProductId != "" {
		amazonProduct, amazonErr := s.getAmazonProductWithCache(ctx, mapping.ProductId, mapping.Region, mapping.TenantId, storeID)
		if amazonErr != nil {
			return nil, fmt.Errorf("获取Amazon产品数据失败: %w", amazonErr)
		}
		pricingCtx.AmazonProduct = amazonProduct
		pricingCtx.ProductID = mapping.ProductId
	}

	// 计算原始成本价
	priceType := s.storeConfig.GetPriceType()
	originCostPrice := s.dataService.CalculateOriginCostPriceWithAmazon(
		mapping,
		supplierPrice,
		pricingCtx.AmazonProduct,
		s.config.UseAmazonPrice,
		priceType,
	)

	if originCostPrice <= 0 {
		return nil, errors.New("无法计算原始成本价")
	}
	pricingCtx.OriginCostPrice = originCostPrice

	// 获取核价规则
	pricingRules, err := s.dataService.GetPricingRules(storeID)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v", err)
	}

	// 根据成本价获取合适的规则
	if len(pricingRules) > 0 {
		pricingCtx.PricingRule = s.ruleCalculator.GetDefaultPricingRules(originCostPrice, &pricingRules)
	}

	// 计算最低可接受价格
	pricingCtx.MinAcceptablePrice = s.ruleCalculator.CalculateMinAcceptablePrice(originCostPrice, pricingCtx.PricingRule)

	return pricingCtx, nil
}

// MakeDecisionForSalesBoost 对销量提升场景的商品做出核价决策
func (s *PricingDecisionService) MakeDecisionForSalesBoost(goods *temupricing.SalesBoostGoods, sku *temupricing.SalesBoostSku, storeID int64) (*temupricing.Decision, error) {
	ctx := context.Background()
	decision := &temupricing.Decision{}

	// 参数校验
	if goods == nil || sku == nil {
		decision.Action = temupricing.DecisionSkip
		decision.Reason = "商品或SKU信息为空"
		return decision, nil
	}

	// 使用配置中的storeID，如果传入的storeID不为0则使用传入的
	targetStoreID := s.config.StoreID
	if storeID != 0 {
		targetStoreID = storeID
	}

	// 解析价格
	currentSupplierPrice := parsePrice(sku.CurrentSupplierPrice.Amount)
	targetSupplierPrice := parsePrice(sku.TargetSupplierPrice.Amount)

	// 由于接口限制，我们需要从其他地方获取tenantID
	tenantID := int64(1) // 默认租户ID，实际应该从配置或上下文获取

	// 构建定价上下文（使用当前供应商价格）
	pricingCtx, err := s.buildPricingContextForSalesBoost(ctx, sku.OutSkuSN, goods.SalesBoostGoodsBasicInfo.GoodsName, currentSupplierPrice, tenantID, targetStoreID)
	if err != nil {
		decision.Action = temupricing.DecisionSkip
		decision.Reason = err.Error()
		s.logger.WithError(err).Warnf("构建销量提升定价上下文失败: %s", goods.SalesBoostGoodsBasicInfo.GoodsName)
		return decision, nil
	}

	// 计算利润率
	if targetSupplierPrice > 0 {
		decision.ProfitMargin = (targetSupplierPrice - pricingCtx.OriginCostPrice) / pricingCtx.OriginCostPrice * 100
	}

	// 记录销量提升定价信息
	s.logSalesBoostPricingInfo(goods, sku, pricingCtx, targetSupplierPrice, decision.ProfitMargin)

	// 执行决策逻辑
	finalDecision := s.makeDecisionByPrice(targetSupplierPrice, pricingCtx.MinAcceptablePrice)

	// 销量提升场景的特殊处理
	if finalDecision.Action == temupricing.DecisionReappeal && !sku.ActionInfo.AllowCreateAppealOrder {
		finalDecision.Action = temupricing.DecisionSkip
		finalDecision.Reason = fmt.Sprintf("目标价格%.2f低于最低可接受价格%.2f，但不允许创建申诉订单(allow_create_appeal_order=false)，保留在售",
			targetSupplierPrice, pricingCtx.MinAcceptablePrice)
	}

	// 设置销量提升特有字段
	finalDecision.TargetPrice = targetSupplierPrice
	finalDecision.TargetMargin = 1.5 // 默认目标利润率
	finalDecision.MinMargin = 1.5    // 默认最小利润率
	finalDecision.AcceptablePrice = pricingCtx.MinAcceptablePrice
	finalDecision.ProfitMargin = decision.ProfitMargin

	return finalDecision, nil
}

// buildPricingContextForSalesBoost 为销量提升场景构建定价上下文
func (s *PricingDecisionService) buildPricingContextForSalesBoost(ctx context.Context, skuSN, goodsName string, supplierPrice float64, _ int64, storeID int64) (*PricingContext, error) {
	pricingCtx := &PricingContext{
		SkuSN:         skuSN,
		GoodsName:     goodsName,
		SupplierPrice: supplierPrice,
		StoreID:       storeID,
	}

	// 获取上架记录映射（使用SKU方式）
	mapping, err := s.dataService.GetProductImportMappingBySku(skuSN, storeID)
	if err != nil {
		return nil, fmt.Errorf("获取上架记录失败: %w", err)
	}
	pricingCtx.Mapping = mapping

	// 获取Amazon产品数据
	if mapping != nil && mapping.ProductId != "" {
		amazonProduct, amazonErr := s.getAmazonProductWithCache(ctx, mapping.ProductId, mapping.Region, mapping.TenantId, storeID)
		if amazonErr != nil {
			return nil, fmt.Errorf("获取Amazon产品数据失败: %w", amazonErr)
		}
		pricingCtx.AmazonProduct = amazonProduct
		pricingCtx.ProductID = mapping.ProductId
	}

	// 计算原始成本价
	priceType := s.storeConfig.GetPriceType()
	originCostPrice := s.dataService.CalculateOriginCostPriceWithAmazon(
		mapping,
		supplierPrice,
		pricingCtx.AmazonProduct,
		s.config.UseAmazonPrice,
		priceType,
	)

	if originCostPrice <= 0 {
		return nil, errors.New("无法计算原始成本价")
	}
	pricingCtx.OriginCostPrice = originCostPrice

	// 获取核价规则
	pricingRules, err := s.dataService.GetPricingRules(storeID)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v", err)
		return nil, errors.New("获取核价规则失败")
	}

	// 根据成本价获取合适的规则
	if len(pricingRules) > 0 {
		pricingCtx.PricingRule = s.ruleCalculator.GetDefaultPricingRules(originCostPrice, &pricingRules)
	}

	// 计算最低可接受价格
	pricingCtx.MinAcceptablePrice = s.ruleCalculator.CalculateMinAcceptablePrice(originCostPrice, pricingCtx.PricingRule)

	return pricingCtx, nil
}
