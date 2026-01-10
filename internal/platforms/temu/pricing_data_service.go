// Package temu 提供TEMU平台核价数据服务功能
package temu

import (
	"fmt"
	"task-processor/internal/domain/model"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// PricingDataService 核价数据服务
type PricingDataService struct {
	managementClient *management.ClientManager
	tenantID         int64
	logger           *logrus.Entry
}

// NewPricingDataService 创建核价数据服务
func NewPricingDataService(managementClient *management.ClientManager, tenantID int64, logger *logrus.Entry) *PricingDataService {
	return &PricingDataService{
		managementClient: managementClient,
		tenantID:         tenantID,
		logger:           logger,
	}
}

// GetPricingRule 获取核价规则
func (s *PricingDataService) GetPricingRule(storeId int64) (*api.PricingRuleRespDTO, error) {
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

// GetProductImportMapping 获取产品上架记录映射
func (s *PricingDataService) GetProductImportMapping(item *Sku, storeId int64) (*api.ProductImportMappingRespDTO, error) {
	mappingClient := s.managementClient.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

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

// GetProductImportMappingBySku 通过SKU获取产品上架记录映射
func (s *PricingDataService) GetProductImportMappingBySku(skuSN string, storeId int64) (*api.ProductImportMappingRespDTO, error) {
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

// CalculateOriginCostPriceWithAmazon 计算原始成本价（支持使用Amazon原始数据）
func (s *PricingDataService) CalculateOriginCostPriceWithAmazon(
	mapping *api.ProductImportMappingRespDTO,
	currentPrice float64,
	amazonProduct *model.Product,
	useAmazonPrice bool,
	priceType string,
) float64 {
	if mapping == nil {
		return 0
	}

	// 如果启用Amazon价格选项且有Amazon产品数据
	if useAmazonPrice && amazonProduct != nil {
		amazonPrice := s.getAmazonPrice(amazonProduct, priceType)

		// 如果获取到了有效的Amazon价格，直接返回
		if amazonPrice > 0 {
			s.logger.Debugf("使用Amazon价格作为成本价 (类型: %s): $%.2f", priceType, amazonPrice)
			return amazonPrice
		}

		s.logger.Debug("Amazon产品数据中没有找到有效的价格，回退到倍数反推逻辑")
	}

	// 回退到原有逻辑：使用售价倍数反推
	if mapping.SalePriceMultiplier != nil && *mapping.SalePriceMultiplier > 0 {
		calculatedPrice := currentPrice / *mapping.SalePriceMultiplier
		s.logger.Debugf("使用售价倍数反推成本价: $%.2f / %.2f = $%.2f",
			currentPrice, *mapping.SalePriceMultiplier, calculatedPrice)
		return calculatedPrice
	}

	return 0
}

// getAmazonPrice 根据价格类型获取Amazon产品价格
func (s *PricingDataService) getAmazonPrice(amazonProduct *model.Product, priceType string) float64 {
	if amazonProduct == nil {
		s.logger.Warn("getAmazonPrice 接收到 nil 产品指针，返回价格 0")
		return 0
	}

	var price float64

	// 根据价格类型获取价格
	switch priceType {
	case "special":
		// 特价，使用最终价格
		price = amazonProduct.FinalPrice
		s.logger.Debugf("使用Amazon特价 (FinalPrice): $%.2f", price)
	case "original":
		// 原价，优先使用list_price，否则使用initial_price
		if amazonProduct.PricesBreakdown.ListPrice != nil && *amazonProduct.PricesBreakdown.ListPrice > 0 {
			price = *amazonProduct.PricesBreakdown.ListPrice
			s.logger.Debugf("使用Amazon原价 (ListPrice): $%.2f", price)
		} else {
			price = amazonProduct.InitialPrice
			s.logger.Debugf("使用Amazon原价 (InitialPrice): $%.2f", price)
		}
	default:
		// 默认使用最终价格
		price = amazonProduct.FinalPrice
		s.logger.Debugf("使用Amazon默认价格 (FinalPrice): $%.2f", price)
	}

	return price
}
