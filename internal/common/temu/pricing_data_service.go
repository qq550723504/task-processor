// Package temu 提供TEMU平台核价数据服务功能
package temu

import (
	"fmt"
	"task-processor/internal/common/management"
	"task-processor/internal/common/management/api"

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

// CalculateOriginCostPrice 计算原始成本价
func (s *PricingDataService) CalculateOriginCostPrice(mapping *api.ProductImportMappingRespDTO, currentPrice float64) float64 {
	if mapping == nil {
		return 0
	}

	// 如果没有成本价，使用售价倍数反推
	if mapping.SalePriceMultiplier != nil && *mapping.SalePriceMultiplier > 0 {
		return currentPrice / *mapping.SalePriceMultiplier
	}

	return 0
}
