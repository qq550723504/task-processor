// Package temu 提供TEMU平台核价决策服务功能
package temu

import (
	"fmt"
	"task-processor/internal/common/management"
	"task-processor/internal/common/management/api"

	"github.com/sirupsen/logrus"
)

// PricingDecisionService 核价决策服务
type PricingDecisionService struct {
	storeID        int64
	storeConfig    *api.StoreRespDTO // 店铺配置缓存
	dataService    *PricingDataService
	ruleCalculator *PricingRuleCalculator
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

// getPriceRejectStrategy 获取核价拒绝策略
func (s *PricingDecisionService) getPriceRejectStrategy() string {
	if s.storeConfig == nil || s.storeConfig.TemuPriceRejectStrategy == "" {
		return "KEEP_ONLINE" // 默认保留在售
	}
	return s.storeConfig.TemuPriceRejectStrategy
}

// MakeDecision 对单个商品做出核价决策
func (s *PricingDecisionService) MakeDecision(item *Sku, storeId int64) (*PricingDecision, error) {
	decision := &PricingDecision{
		Sku: item,
	}

	// 参数校验
	if item == nil {
		decision.Action = DecisionSkip
		decision.Reason = "商品信息为空"
		return decision, nil
	}

	// 从上架记录映射获取原始成本价
	mapping, err := s.dataService.GetProductImportMapping(item, storeId)
	if err != nil {
		decision.Action = DecisionSkip
		decision.Reason = fmt.Sprintf("获取上架记录失败: %v", err)
		s.logger.Warnf("获取商品 %s 的上架记录失败: %v", item.GoodsName, err)
		return decision, nil
	}

	// 计算原始成本价
	originCostPrice := s.dataService.CalculateOriginCostPrice(mapping, item.SupplierPrice)
	if originCostPrice <= 0 {
		decision.Action = DecisionSkip
		decision.Reason = "无法计算原始成本价"
		s.logger.Warnf("商品 %s 无法计算原始成本价，跳过", item.GoodsName)
		return decision, nil
	}

	// 获取核价规则并计算最低可接受价格
	pricingRule, err := s.dataService.GetPricingRule(storeId)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v", err)
	}

	minAcceptablePrice := s.ruleCalculator.CalculateMinAcceptablePrice(originCostPrice, pricingRule)

	s.logger.Infof("商品 %s: SKU=%s, 原始成本=%.2f, 平台推荐价=%.2f, 最低可接受价=%.2f",
		item.GoodsName, item.SkuSN, originCostPrice, item.SupplierPrice, minAcceptablePrice)

	// 执行决策逻辑
	return s.makeDecisionByPrice(item.SupplierPrice, minAcceptablePrice), nil
}

// MakeDecisionForSalesBoost 对销量提升场景的商品做出核价决策
func (s *PricingDecisionService) MakeDecisionForSalesBoost(goods *SalesBoostGoods, sku *SalesBoostSku, storeId int64) (*PricingDecision, error) {
	decision := &PricingDecision{}

	// 参数校验
	if goods == nil || sku == nil {
		decision.Action = DecisionSkip
		decision.Reason = "商品或SKU信息为空"
		return decision, nil
	}

	// 从上架记录映射获取原始成本价
	mapping, err := s.dataService.GetProductImportMappingBySku(sku.OutSkuSN, storeId)
	if err != nil {
		decision.Action = DecisionSkip
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
	originCostPrice := s.dataService.CalculateOriginCostPrice(mapping, currentSupplierPrice)
	if originCostPrice <= 0 {
		decision.Action = DecisionSkip
		decision.Reason = "无法计算原始成本价"
		s.logger.Warnf("商品 %s SKU %s 无法计算原始成本价，跳过",
			goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
		return decision, nil
	}

	// 获取核价规则并计算最低可接受价格
	pricingRule, err := s.dataService.GetPricingRule(storeId)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v", err)
	}

	minAcceptablePrice := s.ruleCalculator.CalculateMinAcceptablePrice(originCostPrice, pricingRule)

	// 计算利润率
	if targetSupplierPrice > 0 {
		decision.ProfitMargin = (targetSupplierPrice - originCostPrice) / originCostPrice * 100
	}

	s.logger.Infof("商品 %s SKU %s: 原始成本=%.2f, 当前价格=%.2f, 目标价格=%.2f, 最低可接受价=%.2f, 利润率=%.2f%%",
		goods.SalesBoostGoodsBasicInfo.GoodsName, sku.OutSkuSN,
		originCostPrice, currentSupplierPrice, targetSupplierPrice, minAcceptablePrice, decision.ProfitMargin)

	// 执行决策逻辑
	finalDecision := s.makeDecisionByPrice(targetSupplierPrice, minAcceptablePrice)

	// 销量提升场景的特殊处理
	if finalDecision.Action == DecisionReappeal && !sku.ActionInfo.AllowCreateAppealOrder {
		finalDecision.Action = DecisionSkip
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
func (s *PricingDecisionService) makeDecisionByPrice(actualPrice, minAcceptablePrice float64) *PricingDecision {
	decision := &PricingDecision{}

	if actualPrice >= minAcceptablePrice {
		decision.Action = DecisionAccept
		decision.Reason = fmt.Sprintf("价格%.2f >= 最低可接受价%.2f，满足要求",
			actualPrice, minAcceptablePrice)
	} else {
		// 根据店铺配置决定拒绝策略
		strategy := s.getPriceRejectStrategy()
		if strategy == "TAKE_OFFLINE" {
			decision.Action = DecisionReject
			decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置执行下架",
				actualPrice, minAcceptablePrice)
		} else {
			// KEEP_ONLINE - 保留在售
			if s.isRebargainEnabled() {
				decision.Action = DecisionReappeal
				decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，根据店铺配置保留在售并重新报价",
					actualPrice, minAcceptablePrice)
			} else {
				decision.Action = DecisionSkip
				decision.Reason = fmt.Sprintf("价格%.2f < 最低可接受价%.2f，店铺未启用重新议价，保留在售",
					actualPrice, minAcceptablePrice)
			}
		}
	}

	return decision
}
