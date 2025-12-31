// Package service 提供成本价提供者的实现。
package service

import (
	"context"
	"fmt"
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/pkg/pricing/model"

	"github.com/sirupsen/logrus"
)

// DefaultCostPriceProvider 默认成本价提供者实现
type DefaultCostPriceProvider struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

// NewDefaultCostPriceProvider 创建默认成本价提供者
func NewDefaultCostPriceProvider(managementClient *management.ClientManager) *DefaultCostPriceProvider {
	return &DefaultCostPriceProvider{
		managementClient: managementClient,
		logger:           logrus.WithField("component", "DefaultCostPriceProvider"),
	}
}

// GetOriginCostPrice 获取原始成本价
func (p *DefaultCostPriceProvider) GetOriginCostPrice(ctx context.Context, productID string, storeID int64) (float64, error) {
	mapping, err := p.GetImportMapping(ctx, productID, storeID)
	if err != nil {
		return 0, fmt.Errorf("获取导入映射失败: %w", err)
	}

	// 优先使用直接的成本价
	if mapping.CostPrice != nil && *mapping.CostPrice > 0 {
		p.logger.Debugf("商品 %s 使用直接成本价: %.2f", productID, *mapping.CostPrice)
		return *mapping.CostPrice, nil
	}

	// 如果没有直接成本价，无法计算
	if mapping.SalePriceMultiplier == nil || *mapping.SalePriceMultiplier <= 0 {
		p.logger.Warnf("商品 %s 缺少成本价和售价倍数信息", productID)
		return 0, model.ErrNoCostPrice
	}

	p.logger.Warnf("商品 %s 缺少成本价，需要通过售价倍数反推", productID)
	return 0, model.ErrNoCostPrice
}

// GetImportMapping 获取商品导入映射
func (p *DefaultCostPriceProvider) GetImportMapping(ctx context.Context, productID string, storeID int64) (*model.ImportMapping, error) {
	mappingClient := p.managementClient.GetProductImportMappingClient()
	if mappingClient == nil {
		return nil, fmt.Errorf("产品导入映射客户端未初始化")
	}

	// 尝试通过平台产品ID获取
	req := &api.ProductImportMappingGetReqDTO{
		PlatformProductId: productID,
	}

	respDto, err := mappingClient.GetProductImportMappingByPlatformProductId(req)
	if err != nil {
		p.logger.Errorf("通过平台产品ID %s 获取导入映射失败: %v", productID, err)

		// 尝试通过SKU获取
		skuReq := &api.ProductImportMappingGetBySkuReqDTO{
			Sku:     productID,
			StoreId: storeID,
		}

		respDto, err = mappingClient.GetProductImportMappingBySku(skuReq)
		if err != nil {
			return nil, fmt.Errorf("获取商品 %s 的导入映射失败: %w", productID, err)
		}
	}

	mapping := &model.ImportMapping{
		CostPrice:           respDto.CostPrice,
		SalePriceMultiplier: respDto.SalePriceMultiplier,
	}

	p.logger.Debugf("获取商品 %s 导入映射成功: 成本价=%v, 售价倍数=%v",
		productID, mapping.CostPrice, mapping.SalePriceMultiplier)

	return mapping, nil
}

// CalculateOriginCostPriceFromCurrentPrice 从当前价格反推原始成本价
func (p *DefaultCostPriceProvider) CalculateOriginCostPriceFromCurrentPrice(ctx context.Context, productID string, storeID int64, currentPrice float64) (float64, error) {
	mapping, err := p.GetImportMapping(ctx, productID, storeID)
	if err != nil {
		return 0, err
	}

	// 优先使用直接的成本价
	if mapping.CostPrice != nil && *mapping.CostPrice > 0 {
		return *mapping.CostPrice, nil
	}

	// 通过售价倍数反推成本价
	if mapping.SalePriceMultiplier != nil && *mapping.SalePriceMultiplier > 0 {
		originCostPrice := currentPrice / *mapping.SalePriceMultiplier
		p.logger.Debugf("商品 %s 通过售价倍数反推成本价: %.2f / %.2f = %.2f",
			productID, currentPrice, *mapping.SalePriceMultiplier, originCostPrice)
		return originCostPrice, nil
	}

	return 0, model.ErrNoCostPrice
}
