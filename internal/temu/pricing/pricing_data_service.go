// package pricing 提供TEMU平台核价数据服务功能
package pricing

import (
	"context"
	"fmt"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// PricingDataService 核价数据服务
type PricingDataService struct {
	runtime         runtime
	pricingRuleRepo pricingRuleLister
	mappingRepo     productImportMappingFinder
	logger          *logrus.Entry
}

type pricingRuleLister interface {
	ListByStoreID(ctx context.Context, storeID int64) ([]listingadmin.PricingRule, error)
}

type productImportMappingFinder interface {
	FindLatest(ctx context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error)
}

// NewPricingDataService 创建核价数据服务
func NewPricingDataService(runtime runtime, logger *logrus.Entry) ProductDataProvider {
	service := &PricingDataService{
		runtime: runtime,
		logger:  logger,
	}
	if runtime != nil {
		service.pricingRuleRepo = runtime.GetLocalPricingRuleRepository()
		service.mappingRepo = runtime.GetLocalProductImportMappingRepository()
	}
	return service
}

// GetPricingRules 获取核价规则（返回数组）
func (s *PricingDataService) GetPricingRules(storeID int64) ([]api.PricingRuleRespDTO, error) {
	if s.pricingRuleRepo != nil {
		rules, err := s.pricingRuleRepo.ListByStoreID(context.Background(), storeID)
		if err != nil {
			s.logger.WithError(err).Warnf("通过本地仓储获取核价规则失败: storeID=%d，回退远程核价规则接口", storeID)
		} else {
			items := make([]api.PricingRuleRespDTO, 0, len(rules))
			for i := range rules {
				items = append(items, pricingRuleDTOFromListingRule(rules[i]))
			}
			enabledRules := s.filterEnabledRules(items)
			s.logger.Infof("成功通过本地仓储获取核价规则: 总数=%d, 启用=%d", len(items), len(enabledRules))
			return enabledRules, nil
		}
	}

	if s.runtime == nil {
		return nil, fmt.Errorf("核价运行时未初始化")
	}

	pricingRuleClient := s.runtime.GetPricingRuleClient()
	if pricingRuleClient == nil {
		return nil, fmt.Errorf("核价规则客户端未初始化")
	}

	req := &api.PricingRuleReqDTO{
		StoreID: &storeID,
	}

	s.logger.Debugf("获取核价规则: storeID=%d", storeID)

	pricingRules, err := pricingRuleClient.GetPricingRule(req)
	if err != nil {
		s.logger.WithError(err).Errorf("获取核价规则失败: storeID=%d", storeID)
		return nil, fmt.Errorf("获取核价规则失败: %w", err)
	}

	// 过滤出启用的规则
	enabledRules := s.filterEnabledRules(pricingRules)

	s.logger.Infof("成功获取核价规则: 总数=%d, 启用=%d", len(pricingRules), len(enabledRules))
	return enabledRules, nil
}

// GetProductImportMapping 获取产品上架记录映射
func (s *PricingDataService) GetProductImportMapping(skuSN string, storeID int64) (*api.ProductImportMappingRespDTO, error) {
	if skuSN == "" {
		return nil, fmt.Errorf("SKU编号不能为空")
	}

	if s.runtime == nil {
		return nil, fmt.Errorf("核价运行时未初始化")
	}

	if s.mappingRepo != nil {
		mapping, err := s.mappingRepo.FindLatest(context.Background(), listingadmin.ProductImportMappingQuery{
			SKU:     skuSN,
			StoreID: &storeID,
		})
		if err != nil {
			s.logger.WithError(err).Warnf("通过本地仓储获取产品导入映射失败: sku=%s, storeID=%d，回退远程产品导入映射接口", skuSN, storeID)
		} else if mapping != nil {
			dto := productImportMappingDTOFromListingMapping(mapping)
			s.logger.Debugf("成功通过本地仓储获取产品导入映射: sku=%s, productID=%s", skuSN, dto.ProductId)
			return dto, nil
		}
	}

	mappingClient := s.runtime.GetProductImportMappingAPI()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

	req := &api.ProductImportMappingGetBySkuReqDTO{
		Sku:     skuSN,
		StoreId: storeID,
	}

	s.logger.Debugf("获取产品导入映射: sku=%s, storeID=%d", skuSN, storeID)

	mapping, err := mappingClient.GetProductImportMappingBySku(req)
	if err != nil {
		s.logger.WithError(err).Errorf("获取产品导入映射失败: sku=%s, storeID=%d", skuSN, storeID)
		return nil, fmt.Errorf("获取产品导入映射失败: %w", err)
	}

	if mapping == nil {
		s.logger.Warnf("未找到产品导入映射: sku=%s, storeID=%d", skuSN, storeID)
		return nil, fmt.Errorf("未找到产品导入映射: sku=%s", skuSN)
	}

	s.logger.Debugf("成功获取产品导入映射: sku=%s, productID=%s", skuSN, mapping.ProductId)
	return mapping, nil
}

// GetProductImportMappingBySku 通过SKU获取产品上架记录映射
func (s *PricingDataService) GetProductImportMappingBySku(skuSN string, storeID int64) (*api.ProductImportMappingRespDTO, error) {
	// 复用GetProductImportMapping的逻辑
	return s.GetProductImportMapping(skuSN, storeID)
}

// CalculateOriginCostPriceWithAmazon 计算原始成本价（支持使用Amazon原始数据）
func (s *PricingDataService) CalculateOriginCostPriceWithAmazon(
	mapping *api.ProductImportMappingRespDTO,
	currentPrice float64,
	amazonProduct *model.Product,
	useAmazonPrice bool,
	priceType string,
) float64 {
	if mapping == nil {
		s.logger.Warn("产品映射为空，无法计算成本价")
		return 0
	}

	if currentPrice <= 0 {
		s.logger.Warn("当前价格无效，无法计算成本价")
		return 0
	}

	// 如果启用Amazon价格选项且有Amazon产品数据
	if useAmazonPrice && amazonProduct != nil {
		amazonPrice := s.getAmazonPrice(amazonProduct, priceType)

		// 如果获取到了有效的Amazon价格，直接返回
		if amazonPrice > 0 {
			s.logger.Infof("使用Amazon价格作为成本价 (类型: %s): $%.2f", priceType, amazonPrice)
			return amazonPrice
		}

		s.logger.Debug("Amazon产品数据中没有找到有效的价格，回退到倍数反推逻辑")
	}

	// 回退到原有逻辑：使用售价倍数反推
	return s.calculatePriceByMultiplier(mapping, currentPrice)
}

// filterEnabledRules 过滤启用的规则
func (s *PricingDataService) filterEnabledRules(rules []api.PricingRuleRespDTO) []api.PricingRuleRespDTO {
	enabledRules := make([]api.PricingRuleRespDTO, 0, len(rules))

	for _, rule := range rules {
		if rule.Status == 0 { // 0表示启用
			enabledRules = append(enabledRules, rule)
			s.logger.Debugf("启用的规则: %s (类型: %s)", rule.Name, rule.RuleType)
		} else {
			s.logger.Debugf("跳过禁用的规则: %s", rule.Name)
		}
	}

	return enabledRules
}

// calculatePriceByMultiplier 使用售价倍数反推成本价
func (s *PricingDataService) calculatePriceByMultiplier(mapping *api.ProductImportMappingRespDTO, currentPrice float64) float64 {
	if mapping.SalePriceMultiplier == nil {
		s.logger.Warn("售价倍数为空，无法反推成本价")
		return 0
	}

	multiplier := *mapping.SalePriceMultiplier
	if multiplier <= 0 {
		s.logger.Warnf("售价倍数无效: %.2f，无法反推成本价", multiplier)
		return 0
	}

	calculatedPrice := currentPrice / multiplier
	s.logger.Infof("使用售价倍数反推成本价: $%.2f / %.2f = $%.2f",
		currentPrice, multiplier, calculatedPrice)

	return calculatedPrice
}

// getAmazonPrice 根据价格类型获取Amazon产品价格
func (s *PricingDataService) getAmazonPrice(amazonProduct *model.Product, priceType string) float64 {
	if amazonProduct == nil {
		s.logger.Warn("Amazon产品数据为空")
		return 0
	}

	var price float64

	// 根据价格类型获取价格
	switch priceType {
	case "special":
		price = amazonProduct.FinalPrice
		s.logger.Debugf("使用Amazon特价 (FinalPrice): $%.2f", price)
	case "original":
		price = s.getAmazonOriginalPrice(amazonProduct)
		s.logger.Debugf("使用Amazon原价: $%.2f", price)
	default:
		price = amazonProduct.FinalPrice
		s.logger.Debugf("使用Amazon默认价格 (FinalPrice): $%.2f", price)
	}

	if price <= 0 {
		s.logger.Warnf("Amazon价格无效: $%.2f", price)
	}

	return price
}

// getAmazonOriginalPrice 获取Amazon原价
func (s *PricingDataService) getAmazonOriginalPrice(amazonProduct *model.Product) float64 {
	// 优先使用list_price
	if amazonProduct.PricesBreakdown.ListPrice != nil && *amazonProduct.PricesBreakdown.ListPrice > 0 {
		price := *amazonProduct.PricesBreakdown.ListPrice
		s.logger.Debugf("使用Amazon ListPrice: $%.2f", price)
		return price
	}

	// 否则使用initial_price
	if amazonProduct.InitialPrice > 0 {
		s.logger.Debugf("使用Amazon InitialPrice: $%.2f", amazonProduct.InitialPrice)
		return amazonProduct.InitialPrice
	}

	s.logger.Warn("Amazon产品没有有效的原价信息")
	return 0
}

func pricingRuleDTOFromListingRule(rule listingadmin.PricingRule) api.PricingRuleRespDTO {
	dto := api.PricingRuleRespDTO{
		ID:         rule.ID,
		Name:       rule.Name,
		RuleCode:   rule.RuleCode,
		StoreID:    rule.StoreID,
		CategoryID: rule.CategoryID,
		RuleType:   rule.RuleType,
		Status:     int(rule.Status),
		TenantID:   rule.TenantID,
	}
	dto.PriceMin = ptrFloat64(rule.PriceMin)
	dto.PriceMax = ptrFloat64(rule.PriceMax)
	dto.RuleValue = ptrFloat64(rule.RuleValue)
	dto.FixedValue = rule.FixedValue
	if rule.Description != "" {
		dto.Description = ptrString(rule.Description)
	}
	if rule.AcceptCondition != "" {
		dto.AcceptCondition = ptrString(rule.AcceptCondition)
	}
	if rule.RejectCondition != "" {
		dto.RejectCondition = ptrString(rule.RejectCondition)
	}
	if rule.Remark != "" {
		dto.Remark = ptrString(rule.Remark)
	}
	return dto
}

func productImportMappingDTOFromListingMapping(mapping *listingadmin.ProductImportMapping) *api.ProductImportMappingRespDTO {
	if mapping == nil {
		return nil
	}
	dto := &api.ProductImportMappingRespDTO{
		ID:                      mapping.ID,
		ImportTaskId:            mapping.ImportTaskID,
		StoreId:                 mapping.StoreID,
		Platform:                mapping.Platform,
		Region:                  mapping.Region,
		ProductId:               mapping.ProductID,
		CostPrice:               mapping.CostPrice,
		FilterRuleId:            mapping.FilterRuleID,
		ProfitRuleId:            mapping.ProfitRuleID,
		SalePriceMultiplier:     ptrFloat64(mapping.SalePriceMultiplier),
		DiscountPriceMultiplier: ptrFloat64(mapping.DiscountPriceMultiplier),
		Status:                  mapping.Status,
		TenantId:                mapping.TenantID,
	}
	if mapping.ParentProductID != "" {
		dto.ParentProductId = ptrString(mapping.ParentProductID)
	}
	if mapping.PlatformProductID != "" {
		dto.PlatformProductId = ptrString(mapping.PlatformProductID)
	}
	if mapping.PlatformParentProductID != "" {
		dto.PlatformParentProductId = ptrString(mapping.PlatformParentProductID)
	}
	if mapping.SKU != "" {
		dto.Sku = ptrString(mapping.SKU)
	}
	if mapping.FilterRuleRange != "" {
		dto.FilterRuleRange = ptrString(mapping.FilterRuleRange)
	}
	if mapping.Remark != "" {
		dto.Remark = ptrString(mapping.Remark)
	}
	return dto
}

func ptrString(value string) *string {
	if value == "" {
		return nil
	}
	out := value
	return &out
}

func ptrFloat64(value float64) *float64 {
	out := value
	return &out
}
