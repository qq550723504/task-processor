package temu

import (
	"fmt"
	"task-processor/internal/common/management"
	"task-processor/internal/common/management/api"

	"github.com/sirupsen/logrus"
)

// PricingDecisionService 核价决策服务
type PricingDecisionService struct {
	managementClient *management.ClientManager
	tenantID         int64
	storeID          int64
	storeConfig      *api.StoreRespDTO // 店铺配置缓存
	logger           *logrus.Entry
}

// NewPricingDecisionService 创建核价决策服务
func NewPricingDecisionService(managementClient *management.ClientManager, tenantID, storeID int64) *PricingDecisionService {
	service := &PricingDecisionService{
		managementClient: managementClient,
		tenantID:         tenantID,
		storeID:          storeID,
		logger: logrus.WithFields(logrus.Fields{
			"component": "PricingDecisionService",
			"tenantID":  tenantID,
			"storeID":   storeID,
		}),
	}

	// 初始化时加载店铺配置
	if err := service.loadStoreConfig(); err != nil {
		service.logger.Warnf("加载店铺配置失败: %v", err)
	}

	return service
}

// loadStoreConfig 加载店铺配置
func (s *PricingDecisionService) loadStoreConfig() error {
	storeClient := s.managementClient.GetStoreClient()
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
	mapping, err := s.getProductImportMapping(item, storeId)
	if err != nil {
		decision.Action = DecisionSkip
		decision.Reason = fmt.Sprintf("获取上架记录失败: %v", err)
		s.logger.Warnf("获取商品 %s 的上架记录失败: %v", item.GoodsName, err)
		return decision, nil
	}

	// 计算原始成本价
	originCostPrice := s.calculateOriginCostPrice(mapping, item.SupplierPrice)
	if originCostPrice <= 0 {
		decision.Action = DecisionSkip
		decision.Reason = "无法计算原始成本价"
		s.logger.Warnf("商品 %s 无法计算原始成本价，跳过", item.GoodsName)
		return decision, nil
	}

	// 通过核价规则获取利润率
	profitRate := 1.5 // 默认利润率
	pricingRule, err := s.getPricingRule(storeId)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v，使用默认利润率%.2f", err, profitRate)
	} else if pricingRule != nil && pricingRule.RuleValue != nil {
		// 使用核价规则中的目标利润率
		profitRate = *pricingRule.RuleValue
		s.logger.Infof("使用核价规则 %s 的目标利润率: %.2f%%", pricingRule.Name, profitRate)
	}

	s.logger.Infof("商品 %s: SKU=%s, 原始成本=%.2f, 平台推荐价=%.2f, 利润率=%.2f%%",
		item.GoodsName, item.SkuSN, originCostPrice, item.SupplierPrice, profitRate)

	minAcceptablePrice := originCostPrice * profitRate
	if item.SupplierPrice >= minAcceptablePrice {
		decision.Action = DecisionAccept
		decision.Reason = fmt.Sprintf("平台推荐价%.2f >= 最低可接受价%.2f，利润率%.2f%%",
			item.SupplierPrice, minAcceptablePrice, profitRate)
	} else {
		// 根据店铺配置决定拒绝策略
		strategy := s.getPriceRejectStrategy()
		if strategy == "TAKE_OFFLINE" {
			decision.Action = DecisionReject
			decision.Reason = fmt.Sprintf("平台推荐价%.2f < 最低可接受价%.2f，根据店铺配置执行下架",
				item.SupplierPrice, minAcceptablePrice)
		} else {
			// KEEP_ONLINE - 保留在售
			if s.isRebargainEnabled() {
				decision.Action = DecisionReappeal
				decision.Reason = fmt.Sprintf("平台推荐价%.2f < 最低可接受价%.2f，根据店铺配置保留在售并重新报价",
					item.SupplierPrice, minAcceptablePrice)
			} else {
				decision.Action = DecisionSkip
				decision.Reason = fmt.Sprintf("平台推荐价%.2f < 最低可接受价%.2f，店铺未启用重新议价，保留在售",
					item.SupplierPrice, minAcceptablePrice)
			}
		}
	}

	return decision, nil
}

// getPricingRule 获取核价规则
func (s *PricingDecisionService) getPricingRule(storeId int64) (*api.PricingRuleRespDTO, error) {
	pricingRuleClient := s.managementClient.GetPricingRuleClient()
	if pricingRuleClient == nil {
		return nil, fmt.Errorf("核价规则客户端未初始化")
	}

	req := &api.PricingRuleReqDTO{
		TenantID: s.tenantID,
		StoreID:  &storeId,
	}

	pricingRule, err := pricingRuleClient.GetPricingRule(req)
	if err != nil {
		return nil, fmt.Errorf("获取核价规则失败: %w", err)
	}

	// 验证核价规则是否启用
	if pricingRule.Status != 0 {
		return nil, fmt.Errorf("核价规则未启用: %s", pricingRule.Name)
	}

	return pricingRule, nil
}

// getProductImportMapping 获取产品上架记录映射
func (s *PricingDecisionService) getProductImportMapping(item *Sku, storeId int64) (*api.ProductImportMappingRespDTO, error) {
	mappingClient := s.managementClient.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

	// 使用SKU SN作为平台产品ID查询
	req := &api.ProductImportMappingGetBySkuReqDTO{
		Sku:     item.SkuSN,
		StoreId: storeId,
	}

	mapping, err := mappingClient.GetProductImportMappingBySku(req)
	if err != nil {
		return nil, fmt.Errorf("获取产品导入映射失败: %w", err)
	}

	return mapping, nil
}

// calculateOriginCostPrice 计算原始成本价
func (s *PricingDecisionService) calculateOriginCostPrice(mapping *api.ProductImportMappingRespDTO, currentPrice float64) float64 {
	if mapping == nil {
		return 0
	}

	// 如果没有成本价，使用售价倍数反推
	if mapping.SalePriceMultiplier != nil && *mapping.SalePriceMultiplier > 0 {
		return currentPrice / *mapping.SalePriceMultiplier
	}

	return 0
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
	mapping, err := s.getProductImportMappingBySku(sku.OutSkuSN, storeId)
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
	originCostPrice := s.calculateOriginCostPrice(mapping, currentSupplierPrice)
	if originCostPrice <= 0 {
		decision.Action = DecisionSkip
		decision.Reason = "无法计算原始成本价"
		s.logger.Warnf("商品 %s SKU %s 无法计算原始成本价，跳过",
			goods.SalesBoostGoodsBasicInfo.GoodsName, sku.SkuID)
		return decision, nil
	}

	// 获取核价规则
	profitRate := 1.5 // 默认利润率
	pricingRule, err := s.getPricingRule(storeId)
	if err != nil {
		s.logger.Warnf("获取核价规则失败: %v，使用默认利润率%.2f", err, profitRate)
	} else if pricingRule != nil && pricingRule.RuleValue != nil {
		profitRate = *pricingRule.RuleValue
		s.logger.Infof("使用核价规则 %s 的目标利润率: %.2f", pricingRule.Name, profitRate)
	}

	// 计算利润率
	if targetSupplierPrice > 0 {
		decision.ProfitMargin = (targetSupplierPrice - originCostPrice) / originCostPrice * 100
	}

	s.logger.Infof("商品 %s SKU %s: 原始成本=%.2f, 当前价格=%.2f, 目标价格=%.2f, 利润率=%.2f%%",
		goods.SalesBoostGoodsBasicInfo.GoodsName, sku.OutSkuSN,
		originCostPrice, currentSupplierPrice, targetSupplierPrice, decision.ProfitMargin)

	// 决策逻辑：如果目标价格能满足利润率要求，则接受；否则根据店铺配置决定
	minAcceptablePrice := originCostPrice * profitRate
	if targetSupplierPrice >= minAcceptablePrice {
		decision.Action = DecisionAccept
		decision.Reason = fmt.Sprintf("目标价格%.2f满足利润率要求(%.2f%%), 最低可接受价格%.2f",
			targetSupplierPrice, profitRate*100, minAcceptablePrice)
	} else {
		// 根据店铺配置决定拒绝策略
		strategy := s.getPriceRejectStrategy()
		if strategy == "TAKE_OFFLINE" {
			decision.Action = DecisionReject
			decision.Reason = fmt.Sprintf("目标价格%.2f低于最低可接受价格%.2f，根据店铺配置执行下架",
				targetSupplierPrice, minAcceptablePrice)
		} else {
			// KEEP_ONLINE - 保留在售
			if s.isRebargainEnabled() {
				// 检查是否允许创建申诉订单
				if !sku.ActionInfo.AllowCreateAppealOrder {
					decision.Action = DecisionSkip
					decision.Reason = fmt.Sprintf("目标价格%.2f低于最低可接受价格%.2f，但不允许创建申诉订单(allow_create_appeal_order=false)，保留在售",
						targetSupplierPrice, minAcceptablePrice)
				} else {
					decision.Action = DecisionReappeal
					decision.Reason = fmt.Sprintf("目标价格%.2f低于最低可接受价格%.2f，根据店铺配置保留在售并重新报价",
						targetSupplierPrice, minAcceptablePrice)
				}
			} else {
				decision.Action = DecisionSkip
				decision.Reason = fmt.Sprintf("目标价格%.2f低于最低可接受价格%.2f，店铺未启用重新议价，保留在售",
					targetSupplierPrice, minAcceptablePrice)
			}
		}
	}

	decision.TargetPrice = targetSupplierPrice
	decision.TargetMargin = profitRate
	decision.MinMargin = profitRate
	decision.AcceptablePrice = minAcceptablePrice

	return decision, nil
}

// getProductImportMappingBySku 通过SKU获取产品上架记录映射
func (s *PricingDecisionService) getProductImportMappingBySku(skuSN string, storeId int64) (*api.ProductImportMappingRespDTO, error) {
	mappingClient := s.managementClient.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

	req := &api.ProductImportMappingGetBySkuReqDTO{
		Sku:     skuSN,
		StoreId: storeId,
	}

	mapping, err := mappingClient.GetProductImportMappingBySku(req)
	if err != nil {
		return nil, fmt.Errorf("获取产品导入映射失败: %w", err)
	}

	return mapping, nil
}
