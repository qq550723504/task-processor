// Package temu 提供TEMU平台核价决策服务功能
package pricing

import (
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/amazon"
	"task-processor/internal/domain/model"
	"task-processor/internal/domain/product"
	"task-processor/internal/pkg/management"
	managementapi "task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/temu/api"
	"task-processor/internal/platforms/temu/api/models"

	"github.com/sirupsen/logrus"
)

// PricingDecisionService 核价决策服务
type PricingDecisionService struct {
	storeID        int64
	storeConfig    *managementapi.StoreRespDTO // 店铺配置缓存
	dataService    *PricingDataService
	ruleCalculator *PricingRuleCalculator
	productFetcher *product.ProductFetcher // 用于获取Amazon产品数据
	useAmazonPrice bool                    // 是否使用Amazon价格数据（从配置读取）
	logger         *logrus.Entry
}

// NewPricingDecisionService 创建核价决策服务
func NewPricingDecisionService(managementClient *management.ClientManager, tenantID, storeID int64) *PricingDecisionService {
	logger := logrus.WithFields(logrus.Fields{
		"component": "PricingDecisionService",
		"tenantID":  tenantID,
		"storeID":   storeID,
	})

	service := &PricingDecisionService{
		storeID:        storeID,
		dataService:    NewPricingDataService(managementClient, tenantID, logger),
		ruleCalculator: NewPricingRuleCalculator(logger),
		useAmazonPrice: true, // 默认值，向后兼容
		logger:         logger,
	}

	// 初始化时加载店铺配置
	if err := service.loadStoreConfig(managementClient); err != nil {
		service.logger.Warnf("加载店铺配置失败: %v", err)
	}

	return service
}

// NewPricingDecisionServiceWithAmazon 创建支持Amazon数据的核价决策服务
func NewPricingDecisionServiceWithAmazon(
	managementClient *management.ClientManager,
	tenantID, storeID int64,
	amazonConfig *config.AmazonConfig,
	amazonProcessor *amazon.AmazonProcessor,
	platformConfig *config.PlatformConfig, // 改为平台配置参数
) *PricingDecisionService {
	logger := logrus.WithFields(logrus.Fields{
		"component": "PricingDecisionService",
		"tenantID":  tenantID,
		"storeID":   storeID,
	})

	// 创建ProductFetcher用于获取Amazon产品数据
	var productFetcher *product.ProductFetcher
	if managementClient != nil && amazonConfig != nil && amazonProcessor != nil {
		rawJsonDataClient := managementClient.GetRawJsonDataClient()
		if rawJsonDataClient != nil {
			productFetcher = product.NewProductFetcher(rawJsonDataClient, amazonConfig, amazonProcessor)
		}
	}

	// 从配置获取useAmazonPrice，如果配置为空则使用默认值true
	useAmazonPrice := true
	if platformConfig != nil {
		useAmazonPrice = platformConfig.AutoPricing.UseAmazonPrice
	}

	service := &PricingDecisionService{
		storeID:        storeID,
		dataService:    NewPricingDataService(managementClient, tenantID, logger),
		ruleCalculator: NewPricingRuleCalculator(logger),
		productFetcher: productFetcher,
		useAmazonPrice: useAmazonPrice, // 使用配置值
		logger:         logger,
	}

	// 初始化时加载店铺配置
	if err := service.loadStoreConfig(managementClient); err != nil {
		service.logger.Warnf("加载店铺配置失败: %v", err)
	}

	return service
}

// loadStoreConfig 加载店铺配置
func (s *PricingDecisionService) loadStoreConfig(managementClient *management.ClientManager) error {
	storeClient := managementClient.GetStoreClient()
	if storeClient == nil {
		return fmt.Errorf("店铺客户端未初始化")
	}

	store, err := storeClient.GetStore(s.storeID)
	if err != nil {
		return fmt.Errorf("获取店铺配置失败: %w", err)
	}

	s.storeConfig = store
	s.logger.Infof("店铺配置加载成功: 重新议价=%v, 核价拒绝策略=%s",
		s.isRebargainEnabled(), s.getPriceRejectStrategy())
	return nil
}

// isRebargainEnabled 是否启用重新议价
func (s *PricingDecisionService) isRebargainEnabled() bool {
	if s.storeConfig == nil || s.storeConfig.EnableRebargain == nil {
		return false
	}
	return *s.storeConfig.EnableRebargain
}

// getPriceType 获取店铺配置的价格类型
func (s *PricingDecisionService) getPriceType() string {
	if s.storeConfig == nil || s.storeConfig.PriceType == "" {
		return "special" // 默认使用特价
	}
	return s.storeConfig.PriceType
}

// getPriceRejectStrategy 获取核价拒绝策略
func (s *PricingDecisionService) getPriceRejectStrategy() string {
	if s.storeConfig == nil || s.storeConfig.TemuPriceRejectStrategy == "" {
		return "KEEP_ONLINE" // 默认保留在售
	}
	return s.storeConfig.TemuPriceRejectStrategy
}

// getAmazonProduct 获取Amazon产品数据
func (s *PricingDecisionService) getAmazonProduct(productID, platform, region string, tenantID, storeID int64) (*model.Product, error) {
	if s.productFetcher == nil {
		return nil, fmt.Errorf("ProductFetcher未初始化，无法获取Amazon产品数据")
	}

	req := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  platform,
		Region:    region,
		ProductID: productID,
		StoreID:   storeID,
	}

	amazonProduct, err := s.productFetcher.FetchProduct(req)
	if err != nil {
		s.logger.Warnf("获取Amazon产品数据失败: %v", err)
		return nil, err
	}

	return amazonProduct, nil
}

// getAmazonProductWithRetry 获取Amazon产品数据（带重试机制）
func (s *PricingDecisionService) getAmazonProductWithRetry(productID, region string, tenantID, storeID int64) (*model.Product, error) {
	if s.productFetcher == nil {
		return nil, fmt.Errorf("ProductFetcher未初始化，无法获取Amazon产品数据")
	}

	req := &product.FetchRequest{
		TenantID:  tenantID,
		Platform:  "Amazon",
		Region:    region,
		ProductID: productID,
		StoreID:   storeID,
	}

	// 最多重试3次
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		amazonProduct, err := s.productFetcher.FetchProduct(req)
		if err == nil {
			s.logger.Debugf("第%d次尝试成功获取Amazon产品数据: %s", attempt, productID)
			return amazonProduct, nil
		}

		s.logger.Warnf("第%d次获取Amazon产品数据失败: %v", attempt, err)

		// 如果不是最后一次尝试，继续重试
		if attempt < maxRetries {
			s.logger.Infof("将进行第%d次重试获取Amazon产品数据: %s", attempt+1, productID)
		}
	}

	// 所有重试都失败了
	return nil, fmt.Errorf("经过%d次重试后仍无法获取Amazon产品数据", maxRetries)
}

// MakeDecision 对单个商品做出核价决策
func (s *PricingDecisionService) MakeDecision(item *models.PricingSku, storeId int64) (*models.PricingDecision, error) {
	decision := &models.PricingDecision{
		Sku: item,
	}

	// 参数校验
	if item == nil {
		decision.Action = models.DecisionSkip
		decision.Reason = "商品信息为空"
		return decision, nil
	}

	// 从上架记录映射获取原始成本价
	mapping, err := s.dataService.GetProductImportMapping(item.SkuSN, storeId)
	if err != nil {
		decision.Action = models.DecisionSkip
		decision.Reason = fmt.Sprintf("获取上架记录失败: %v", err)
		s.logger.Warnf("获取商品 %s 的上架记录失败: %v", item.GoodsName, err)
		return decision, nil
	}

	// 计算原始成本价
	// 尝试获取Amazon产品数据来使用原始价格
	var amazonProduct *model.Product

	if mapping != nil && mapping.ProductId != "" {
		// 从mapping中获取Amazon产品ID和区域信息
		amazonProduct, err = s.getAmazonProductWithRetry(mapping.ProductId, mapping.Region,
			mapping.TenantId, storeId)
		if err != nil {
			decision.Action = models.DecisionSkip
			decision.Reason = fmt.Sprintf("获取Amazon产品数据失败: %v", err)
			s.logger.Errorf("商品 %s 获取Amazon产品数据失败，跳过处理: %v", item.GoodsName, err)
			return decision, nil
		} else if amazonProduct != nil {
			s.logger.Debugf("成功获取Amazon产品数据: %s", mapping.ProductId)
		}
	}

	// 获取店铺配置的价格类型
	priceType := s.getPriceType()

	originCostPrice := s.dataService.CalculateOriginCostPriceWithAmazon(
		mapping,
		item.SupplierPrice,
		amazonProduct,
		s.useAmazonPrice, // 使用配置值而不是硬编码
		priceType,
	)
	if originCostPrice <= 0 {
		decision.Action = models.DecisionSkip
		decision.Reason = "无法计算原始成本价"
		s.logger.Warnf("商品 %s 无法计算原始成本价，跳过", item.GoodsName)
		return decision, nil
	}

	// 获取核价规则并计算最低可接受价格
	pricingRules, err := s.dataService.GetPricingRules(storeId)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v", err)
	}

	// 根据成本价获取合适的规则
	var pricingRule *managementapi.PricingRuleRespDTO
	if len(pricingRules) > 0 {
		pricingRule = s.ruleCalculator.GetDefaultPricingRules(originCostPrice, &pricingRules)
	}

	minAcceptablePrice := s.ruleCalculator.CalculateMinAcceptablePrice(originCostPrice, pricingRule)

	s.logger.Infof("商品 %s: SKU=%s, 原始成本=%.2f, 平台推荐价=%.2f, 最低可接受价=%.2f",
		item.GoodsName, item.SkuSN, originCostPrice, item.SupplierPrice, minAcceptablePrice)

	// 执行决策逻辑
	return s.makeDecisionByPrice(item.SupplierPrice, minAcceptablePrice), nil
}

// MakeDecisionForSalesBoost 对销量提升场景的商品做出核价决策
func (s *PricingDecisionService) MakeDecisionForSalesBoost(goods *api.SalesBoostGoods, sku *models.SalesBoostSku, storeId int64) (*models.PricingDecision, error) {
	decision := &models.PricingDecision{}

	// 参数校验
	if goods == nil || sku == nil {
		decision.Action = models.DecisionSkip
		decision.Reason = "商品或SKU信息为空"
		return decision, nil
	}

	// 从上架记录映射获取原始成本价
	mapping, err := s.dataService.GetProductImportMappingBySku(sku.OutSkuSN, storeId)
	if err != nil {
		decision.Action = models.DecisionSkip
		decision.Reason = fmt.Sprintf("获取上架记录失败: %v", err)
		s.logger.Warnf("获取商品 %s SKU %s 的上架记录失败: %v",
			goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID, err)
		return decision, nil
	}

	// 解析当前供应商价格
	var currentSupplierPrice, targetSupplierPrice float64
	if sku.CurrentSupplierPrice.Amount != "" {
		currentSupplierPrice = parsePrice(sku.CurrentSupplierPrice.Amount)
	}
	if sku.TargetSupplierPrice.Amount != "" {
		targetSupplierPrice = parsePrice(sku.TargetSupplierPrice.Amount)
	}

	// 计算原始成本价
	// 尝试获取Amazon产品数据来使用原始价格
	var amazonProduct *model.Product

	if mapping != nil && mapping.ProductId != "" {
		// 从mapping中获取Amazon产品ID和区域信息
		amazonProduct, err = s.getAmazonProductWithRetry(mapping.ProductId, mapping.Region,
			mapping.TenantId, storeId)
		if err != nil {
			decision.Action = models.DecisionSkip
			decision.Reason = fmt.Sprintf("获取Amazon产品数据失败: %v", err)
			s.logger.Errorf("商品ID %s SKU %s 获取Amazon产品数据失败，跳过处理: %v",
				goods.SalesBoostGoodsBasicInfo.GoodsID, sku.SkuID, err)
			return decision, nil
		} else if amazonProduct != nil {
			s.logger.Debugf("成功获取Amazon产品数据: %s", mapping.ProductId)
		}
	}

	// 获取店铺配置的价格类型
	priceType := s.getPriceType()

	originCostPrice := s.dataService.CalculateOriginCostPriceWithAmazon(
		mapping,
		currentSupplierPrice,
		amazonProduct,
		s.useAmazonPrice, // 使用配置值而不是硬编码
		priceType,
	)
	if originCostPrice <= 0 {
		decision.Action = models.DecisionSkip
		decision.Reason = "无法计算原始成本价"
		s.logger.Warnf("商品ID %s SKU %s 无法计算原始成本价，跳过",
			goods.SalesBoostGoodsBasicInfo.GoodsID, sku.SkuID)
		return decision, nil
	}

	// 获取核价规则并计算最低可接受价格
	pricingRules, err := s.dataService.GetPricingRules(storeId)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v", err)
	}

	// 根据成本价获取合适的规则
	var pricingRule *managementapi.PricingRuleRespDTO
	if len(pricingRules) > 0 {
		pricingRule = s.ruleCalculator.GetDefaultPricingRules(originCostPrice, &pricingRules)
	}

	minAcceptablePrice := s.ruleCalculator.CalculateMinAcceptablePrice(originCostPrice, pricingRule)

	// 计算利润率
	if targetSupplierPrice > 0 {
		decision.ProfitMargin = (targetSupplierPrice - originCostPrice) / originCostPrice * 100
	}

	s.logger.Infof("商品ID %s SKU %s: 原始成本=%.2f, 当前价格=%.2f, 目标价格=%.2f, 最低可接受价=%.2f, 利润率=%.2f%%",
		goods.SalesBoostGoodsBasicInfo.GoodsID, sku.OutSkuSN,
		originCostPrice, currentSupplierPrice, targetSupplierPrice, minAcceptablePrice, decision.ProfitMargin)

	// 执行决策逻辑
	finalDecision := s.makeDecisionByPrice(targetSupplierPrice, minAcceptablePrice)

	// 销量提升场景的特殊处理
	if finalDecision.Action == models.DecisionReappeal && !sku.ActionInfo.AllowCreateAppealOrder {
		finalDecision.Action = models.DecisionSkip
		finalDecision.Reason = fmt.Sprintf("目标价格%.2f低于最低可接受价格%.2f，但不允许创建申诉订单(allow_create_appeal_order=false)，保留在售",
			targetSupplierPrice, minAcceptablePrice)
	}

	// 设置销量提升特有字段
	finalDecision.TargetPrice = targetSupplierPrice
	finalDecision.TargetMargin = 1.5 // 默认目标利润率
	finalDecision.MinMargin = 1.5    // 默认最小利润率
	finalDecision.AcceptablePrice = minAcceptablePrice

	return finalDecision, nil
}

// makeDecisionByPrice 根据价格做出决策
func (s *PricingDecisionService) makeDecisionByPrice(actualPrice, minAcceptablePrice float64) *models.PricingDecision {
	decision := &models.PricingDecision{}

	if actualPrice >= minAcceptablePrice {
		decision.Action = models.DecisionAccept
		decision.Reason = fmt.Sprintf("价格%.2f >= 最低可接受价%.2f，满足要求",
			actualPrice, minAcceptablePrice)
	} else {
		// 根据店铺配置决定拒绝策略
		strategy := s.getPriceRejectStrategy()
		if strategy == "TAKE_OFFLINE" {
			decision.Action = models.DecisionReject
			decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置执行下架",
				actualPrice, minAcceptablePrice)
		} else {
			// KEEP_ONLINE - 保留在售
			if s.isRebargainEnabled() {
				decision.Action = models.DecisionReappeal
				decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置保留在售并重新报价",
					actualPrice, minAcceptablePrice)
			} else {
				decision.Action = models.DecisionSkip
				decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，店铺未启用重新议价，保留在售",
					actualPrice, minAcceptablePrice)
			}
		}
	}

	return decision
}

// parsePrice 解析价格字符串为浮点数
func parsePrice(price string) float64 {
	if price == "" {
		return 0.0
	}
	var result float64
	fmt.Sscanf(price, "%f", &result)
	return result
}
