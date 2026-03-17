// Package pricing 提供 SHEIN 平台定价功能
package pricing

import (
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/shein/api/pricing"
)

// calculateReappealPrices 计算重新议价的价格
func (s *autoPricingServiceImpl) calculateReappealPrices(product *pricing.BargainPageData, rules []managementapi.PricingRuleRespDTO) map[string]float64 {
	newPrices := make(map[string]float64)
	mappingClient := s.managementClient.GetProductImportMappingClient()

	for _, sku := range product.SkuCostPrices {
		if len(sku.CostPriceHistories) == 0 {
			continue
		}

		// 获取产品映射信息
		reqDto := &managementapi.ProductImportMappingGetReqDTO{PlatformProductId: sku.SkuCode}
		respDto, err := mappingClient.GetProductImportMappingByPlatformProductId(reqDto)
		if err != nil {
			s.logger.Errorf("获取产品导入映射关系失败: %v", err)
			continue
		}

		// 计算原始价格
		var originPrice float64
		if respDto.CostPrice != nil {
			originPrice = *respDto.CostPrice
		} else {
			if respDto.SalePriceMultiplier != nil && *respDto.SalePriceMultiplier != 0 {
				originPrice = sku.CostPriceHistories[0].CostPrice / *respDto.SalePriceMultiplier
			} else {
				originPrice = 0
			}
		}

		if originPrice == 0 {
			s.logger.Warnf("[%s] 获取原始价格失败, 跳过", sku.SkuCode)
			continue
		}

		// 计算通过价格（作为新的议价价格）
		passPrice := s.calculator.GetAutoPrice(originPrice, rules)
		newPrices[sku.SkuCode] = passPrice

		s.logger.Debugf("SKU: %s, 原价: %.2f, 建议价: %.2f, 新议价: %.2f",
			sku.SkuCode, originPrice, sku.SuggestCostPrice, passPrice)
	}

	return newPrices
}
