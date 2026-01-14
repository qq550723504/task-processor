// Package scheduler 提供SHEIN平台调度器相关服务
package scheduler

import (
	"task-processor/internal/platforms/shein/api/pricing"
)

// PricingRequestBuilder 核价请求构造器
type PricingRequestBuilder struct{}

// NewPricingRequestBuilder 创建核价请求构造器
func NewPricingRequestBuilder() *PricingRequestBuilder {
	return &PricingRequestBuilder{}
}

// BuildAcceptRequest 构造接受报价请求
func (b *PricingRequestBuilder) BuildAcceptRequest(product *pricing.BargainPageData) *pricing.BatchHandleCostDiscussRequest {
	return &pricing.BatchHandleCostDiscussRequest{
		ConfirmInfos: &[]pricing.ConfirmInfo{
			{
				DiscussAuditType: 1, // 1表示接受
				DiscussSn:        product.BargainSn,
				DocumentSn:       product.DocumentSn,
			},
		},
	}
}

// BuildRejectRequest 构造拒绝报价请求
func (b *PricingRequestBuilder) BuildRejectRequest(product *pricing.BargainPageData) *pricing.BatchHandleCostDiscussRequest {
	return &pricing.BatchHandleCostDiscussRequest{
		ConfirmInfos: &[]pricing.ConfirmInfo{
			{
				DiscussAuditType: 2, // 2表示拒绝
				DiscussSn:        product.BargainSn,
				DocumentSn:       product.DocumentSn,
			},
		},
	}
}

// BuildReappealRequest 构造重新议价请求
func (b *PricingRequestBuilder) BuildReappealRequest(
	product *pricing.BargainPageData,
	newPrices map[string]float64,
	reason string,
) *pricing.BatchHandleCostDiscussRequest {
	// 构造SKU成本信息列表
	skuCostInfoList := make([]pricing.SkuCostInfoForReappeal, 0, len(product.SkuCostPrices))

	for _, sku := range product.SkuCostPrices {
		// 获取新报价
		newPrice, exists := newPrices[sku.SkuCode]
		if !exists {
			continue
		}

		// 获取上次成本价
		var lastCost float64
		var lastCurrency string
		if len(sku.CostPriceHistories) > 0 {
			lastCost = sku.CostPriceHistories[0].CostPrice
			lastCurrency = sku.CostPriceHistories[0].Currency
		}

		skuCostInfoList = append(skuCostInfoList, pricing.SkuCostInfoForReappeal{
			Cost:         newPrice,
			Currency:     sku.SuggestCostCurrency,
			LastCost:     lastCost,
			LastCurrency: lastCurrency,
			SkuCode:      sku.SkuCode,
		})
	}

	// 计算当前是第几次议价
	// 最多可以议价4次，AppealCount是剩余次数
	// DiscussStep = 总次数(4) - 剩余次数(AppealCount) + 1
	discussStep := 4 - product.AppealCount + 1

	// 构造重新议价请求
	return &pricing.BatchHandleCostDiscussRequest{
		CreateCostDiscusses: &[]pricing.CreateCostDiscuss{
			{
				DiscussSn:       product.BargainSn,
				DiscussStep:     discussStep,
				DocumentSn:      product.DocumentSn,
				SkcName:         product.SkcName,
				Reason:          reason,
				FileUploadList:  []string{},
				SkuCostInfoList: skuCostInfoList,
			},
		},
	}
}
