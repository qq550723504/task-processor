// Package pricing 提供 SHEIN 平台定价功能
package pricing

import (
	"context"
	"fmt"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/shein/api/pricing"
)

// evaluateProduct 评估单个产品
func (s *autoPricingServiceImpl) evaluateProduct(product *pricing.BargainPageData, rules []listingruntime.PricingRule, storeID int64, enableRebargain bool) PricingDecision {
	// 检查是否有SKU成本价数据
	if len(product.SkuCostPrices) == 0 {
		return PricingDecision{
			Product:      *product,
			Action:       "skip",
			Reason:       "无成本价数据",
			BatchReQuote: nil,
		}
	}

	// 检查所有SKU是否都符合通过条件
	allSKUsPass, shouldSkip := s.checkAllSKUsPassCondition(product.SkuCostPrices, rules, storeID)
	if shouldSkip {
		return PricingDecision{
			Product:      *product,
			Action:       "skip",
			Reason:       "没有有效的SKU可以处理",
			BatchReQuote: nil,
		}
	}

	if allSKUsPass {
		// 所有SKU都通过，同意报价(使用新接口)
		return PricingDecision{
			Product:      *product,
			Action:       "accept",
			Reason:       "所有SKU都符合通过条件",
			BatchReQuote: s.requestBuilder.BuildAcceptRequest(product),
		}
	} else {
		// 有SKU不通过，根据配置决定是拒绝还是重新议价
		if enableRebargain && product.AppealCount > 0 {
			// 启用重新议价且还有剩余次数，计算新价格并重新议价
			newPrices := s.calculateReappealPrices(product, rules, storeID)
			return PricingDecision{
				Product:      *product,
				Action:       "reappeal",
				Reason:       fmt.Sprintf("价格不符合条件，重新议价（剩余次数：%d）", product.AppealCount),
				BatchReQuote: s.requestBuilder.BuildReappealRequest(product, newPrices, "成本高"),
			}
		} else {
			// 不启用重新议价或没有剩余次数，直接拒绝
			reason := "有SKU不符合通过条件"
			if enableRebargain && product.AppealCount == 0 {
				reason = "价格不符合条件且无剩余议价次数"
			}
			return PricingDecision{
				Product:      *product,
				Action:       "reject",
				Reason:       reason,
				BatchReQuote: s.requestBuilder.BuildRejectRequest(product),
			}
		}
	}
}

// checkAllSKUsPassCondition 检查所有SKU是否都符合通过条件
func (s *autoPricingServiceImpl) checkAllSKUsPassCondition(skus []pricing.SkuCostPrice, rules []listingruntime.PricingRule, storeID int64) (allPass bool, shouldSkip bool) {
	passedSKUCount := 0
	totalSKUCount := 0

	for _, sku := range skus {
		if len(sku.CostPriceHistories) == 0 {
			continue
		}

		totalSKUCount++

		respDto, err := s.getProductImportMappingByPlatformProductID(sku.SkuCode, storeID)
		if err != nil {
			s.logger.Errorf("获取产品导入映射关系失败: %v", err)
			return false, true
		}

		var originPrice float64
		if respDto.CostPrice > 0 {
			originPrice = respDto.CostPrice
		} else {
			if respDto.SalePriceMultiplier != nil {
				if *respDto.SalePriceMultiplier != 0 {
					originPrice = sku.CostPriceHistories[0].CostPrice / *respDto.SalePriceMultiplier
				} else {
					originPrice = 0
				}
			} else {
				originPrice = 0
			}
		}
		suggestPrice := sku.SuggestCostPrice

		if originPrice == 0 {
			s.logger.Warnf("[%s] 获取原始价格失败, 跳过", sku.SkuCode)
			continue
		}

		// 获取通过价格
		passPrice := s.calculator.GetAutoPrice(originPrice, rules)

		if suggestPrice < passPrice {
			s.logger.Debugf("SKU: %s, 建议价格: %.2f, 低于通过价格: %.2f, 不符合条件",
				sku.SkuCode, suggestPrice, passPrice)
			return false, false
		}

		passedSKUCount++
	}

	if totalSKUCount == 0 {
		return false, true
	}

	return passedSKUCount == totalSKUCount, false
}

func (s *autoPricingServiceImpl) getProductImportMappingByPlatformProductID(platformProductID string, storeID int64) (*listingruntime.ProductImportMapping, error) {
	if s.mappingRepo != nil {
		mapping, err := s.mappingRepo.FindLatest(context.Background(), listingadmin.ProductImportMappingQuery{
			PlatformProductID: platformProductID,
			StoreID:           &storeID,
		})
		if err != nil {
			s.logger.WithError(err).Warnf("通过本地仓储获取SHEIN产品导入映射失败: platformProductID=%s", platformProductID)
		} else if mapping != nil {
			return sheinProductImportMappingFromListing(mapping), nil
		}
	}
	return nil, fmt.Errorf("product import mapping repository is not configured")
}
