// Package pricingsvc 提供TEMU平台核价数据服务功能
package pricingsvc

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// PricingDataService 核价数据服务
type PricingDataService struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewPricingDataService 创建核价数据服务
func NewPricingDataService(managementClient *management.ClientManager, logger *logrus.Entry) ProductDataProvider {
	return &PricingDataService{
		managementClient: managementClient,
		logger:           logger,
	}
}

// GetPricingRules 获取核价规则（返回数组）
func (s *PricingDataService) GetPricingRules(storeID int64) ([]api.PricingRuleRespDTO, error) {
	if s.managementClient == nil {
		return nil, fmt.Errorf("管理客户端未初始化")
	}

	pricingRuleClient := s.managementClient.GetPricingRuleClient()
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

	if s.managementClient == nil {
		return nil, fmt.Errorf("管理客户端未初始化")
	}

	mappingClient := s.managementClient.GetProductImportMappingClient()
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
