// Package pricing 提供TEMU平台核价决策服务功能
package pricing

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	temupricing "task-processor/internal/platforms/temu/api/pricing"

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
	productFetcher *product.ProductFetcher
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
	Data      interface{}
	ExpiresAt time.Time
}

// PricingContext 定价上下文
type PricingContext struct {
	ProductID          string
	SkuSN              string
	GoodsName          string
	SupplierPrice      float64
	StoreID            int64
	Mapping            *managementapi.ProductImportMappingRespDTO
	AmazonProduct      *model.Product
	OriginCostPrice    float64
	PricingRule        *managementapi.PricingRuleRespDTO
	MinAcceptablePrice float64
}

// NewPricingDecisionService 创建核价决策服务
func NewPricingDecisionService(
	managementClient *management.ClientManager,
	storeID int64,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
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

	return newPricingDecisionServiceWithConfig(managementClient, config, amazonConfig, amazonProcessor)
}

// newPricingDecisionServiceWithConfig 内部构造函数
func newPricingDecisionServiceWithConfig(
	managementClient *management.ClientManager,
	config *ServiceConfig,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
) (DecisionMaker, error) {
	if managementClient == nil {
		return nil, errors.New("managementClient不能为空")
	}
	if config == nil {
		return nil, errors.New("config不能为空")
	}

	logger := logrus.WithFields(logrus.Fields{
		"component": "PricingDecisionService",
		"storeID":   config.StoreID,
	})

	// 创建依赖服务
	storeConfig, err := NewStoreConfigService(config.StoreID, managementClient)
	if err != nil {
		return nil, fmt.Errorf("创建店铺配置服务失败: %w", err)
	}

	dataService := NewPricingDataService(managementClient, logger)
	ruleCalculator := NewPricingRuleCalculator(logger)

	// 创建ProductFetcher用于获取Amazon产品数据
	var productFetcher *product.ProductFetcher
	if amazonConfig != nil && amazonProcessor != nil {
		rawJsonDataClient := managementClient.GetRawJsonDataAdapter()
		if rawJsonDataClient != nil {
			productFetcher = product.NewProductFetcher(rawJsonDataClient, amazonConfig, amazonProcessor)
		}
	}

	return &PricingDecisionService{
		config:         config,
		storeConfig:    storeConfig,
		dataService:    dataService,
		ruleCalculator: ruleCalculator,
		productFetcher: productFetcher,
		logger:         logger,
	}, nil
}

// getFromCache 从缓存获取数据
func (s *PricingDecisionService) getFromCache(key CacheKey) (interface{}, bool) {
	if item, ok := s.cache.Load(key); ok {
		cacheItem := item.(CacheItem)
		if time.Now().Before(cacheItem.ExpiresAt) {
			return cacheItem.Data, true
		}
		s.cache.Delete(key)
	}
	return nil, false
}

// setCache 设置缓存
func (s *PricingDecisionService) setCache(key CacheKey, data interface{}) {
	item := CacheItem{
		Data:      data,
		ExpiresAt: time.Now().Add(s.config.CacheTimeout),
	}
	s.cache.Store(key, item)
}

// getAmazonProductWithCache 获取Amazon产品数据（带缓存）
func (s *PricingDecisionService) getAmazonProductWithCache(ctx context.Context, productID, region string, tenantID, storeID int64) (*model.Product, error) {
	if s.productFetcher == nil {
		return nil, errors.New("ProductFetcher未初始化，无法获取Amazon产品数据")
	}

	// 尝试从缓存获取
	cacheKey := CacheKey{
		Type:      "amazon_product",
		ProductID: fmt.Sprintf("%s_%s_%d_%d", productID, region, tenantID, storeID),
		StoreID:   storeID,
	}

	if cached, found := s.getFromCache(cacheKey); found {
		if product, ok := cached.(*model.Product); ok {
			s.logger.Debugf("从缓存获取Amazon产品数据: %s", productID)
			return product, nil
		}
	}

	req := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: productID,
		StoreID:   storeID,
	}

	// 带重试机制获取数据
	var amazonProduct *model.Product
	var lastErr error

	for attempt := 1; attempt <= s.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		amazonProduct, lastErr = s.productFetcher.FetchProduct(req)
		if lastErr == nil {
			s.logger.Debugf("第%d次尝试成功获取Amazon产品数据: %s", attempt, productID)
			// 缓存成功结果
			s.setCache(cacheKey, amazonProduct)
			return amazonProduct, nil
		}

		s.logger.Warnf("第%d次获取Amazon产品数据失败: %v", attempt, lastErr)
		if attempt < s.config.MaxRetries {
			s.logger.Infof("将进行第%d次重试获取Amazon产品数据: %s", attempt+1, productID)
			// 简单的退避策略
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("经过%d次重试后仍无法获取Amazon产品数据: %w", s.config.MaxRetries, lastErr)
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
func (s *PricingDecisionService) buildPricingContext(ctx context.Context, skuSN, goodsName string, supplierPrice float64, tenantID, storeID int64) (*PricingContext, error) {
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
		amazonProduct, err := s.getAmazonProductWithCache(ctx, mapping.ProductId, mapping.Region, mapping.TenantId, storeID)
		if err != nil {
			return nil, fmt.Errorf("获取Amazon产品数据失败: %w", err)
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

// logPricingInfo 记录定价信息
func (s *PricingDecisionService) logPricingInfo(pricingCtx *PricingContext) {
	s.logger.WithFields(logrus.Fields{
		"goods_name":           pricingCtx.GoodsName,
		"sku_sn":               pricingCtx.SkuSN,
		"origin_cost_price":    pricingCtx.OriginCostPrice,
		"supplier_price":       pricingCtx.SupplierPrice,
		"min_acceptable_price": pricingCtx.MinAcceptablePrice,
	}).Info("定价信息")
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
func (s *PricingDecisionService) buildPricingContextForSalesBoost(ctx context.Context, skuSN, goodsName string, supplierPrice float64, tenantID, storeID int64) (*PricingContext, error) {
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
		amazonProduct, err := s.getAmazonProductWithCache(ctx, mapping.ProductId, mapping.Region, mapping.TenantId, storeID)
		if err != nil {
			return nil, fmt.Errorf("获取Amazon产品数据失败: %w", err)
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

// logSalesBoostPricingInfo 记录销量提升定价信息
func (s *PricingDecisionService) logSalesBoostPricingInfo(goods *temupricing.SalesBoostGoods, sku *temupricing.SalesBoostSku, pricingCtx *PricingContext, targetPrice, profitMargin float64) {
	s.logger.WithFields(logrus.Fields{
		"goods_id":             goods.SalesBoostGoodsBasicInfo.GoodsID,
		"sku_sn":               sku.OutSkuSN,
		"origin_cost_price":    pricingCtx.OriginCostPrice,
		"current_price":        pricingCtx.SupplierPrice,
		"target_price":         targetPrice,
		"min_acceptable_price": pricingCtx.MinAcceptablePrice,
		"profit_margin":        profitMargin,
	}).Info("销量提升定价信息")
}

// makeDecisionByPrice 根据价格做出决策
func (s *PricingDecisionService) makeDecisionByPrice(actualPrice, minAcceptablePrice float64) *temupricing.Decision {
	decision := &temupricing.Decision{}

	if actualPrice >= minAcceptablePrice {
		decision.Action = temupricing.DecisionAccept
		decision.Reason = fmt.Sprintf("价格%.2f >= 最低可接受价%.2f，满足要求",
			actualPrice, minAcceptablePrice)
	} else {
		// 根据店铺配置决定拒绝策略
		strategy := s.storeConfig.GetPriceRejectStrategy()
		if strategy == "TAKE_OFFLINE" {
			decision.Action = temupricing.DecisionReject
			decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置执行下架",
				actualPrice, minAcceptablePrice)
		} else {
			// KEEP_ONLINE - 保留在售
			if s.storeConfig.IsRebargainEnabled() {
				decision.Action = temupricing.DecisionReappeal
				decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置保留在售并重新报价",
					actualPrice, minAcceptablePrice)
			} else {
				decision.Action = temupricing.DecisionSkip
				decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，店铺未启用重新议价，保留在售",
					actualPrice, minAcceptablePrice)
			}
		}
	}

	return decision
}

// parsePrice 解析价格字符串为浮点数
// 已废弃: 请使用 strutil.ParseFloat
func parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}

	result, err := strconv.ParseFloat(price, 64)
	if err != nil {
		// 如果解析失败，尝试使用fmt.Sscanf作为备选方案
		fmt.Sscanf(price, "%f", &result)
	}
	return result
}
