// Package scheduler 提供SHEIN平台调度器相关服务
package operation

import (
	"task-processor/internal/platforms/shein/api/pricing"
)

// PricingRequestBuilder 核价请求构造器
type PricingRequestBuilder struct{}

// NewPricingRequestBuilder 创建核价请求构造器
func NewPricingRequestBuilder() *PricingRequestBuilder {
	return &PricingRequestBuilder{}
}

// BuildAcceptRequest 构造接受报价请求(使用新接口)
func (b *PricingRequestBuilder) BuildAcceptRequest(product *pricing.BargainPageData) *pricing.BatchReQuoteRequest {
	return &pricing.BatchReQuoteRequest{
		ConfirmInfos: []pricing.ReQuoteConfirmInfo{
			{
				DiscussAuditType: 1, // 1表示接受
				DiscussSn:        product.BargainSn,
				DocumentSn:       product.DocumentSn,
			},
		},
	}
}

// BuildRejectRequest 构造拒绝报价请求(使用新接口)
func (b *PricingRequestBuilder) BuildRejectRequest(product *pricing.BargainPageData) *pricing.BatchReQuoteRequest {
	return &pricing.BatchReQuoteRequest{
		ConfirmInfos: []pricing.ReQuoteConfirmInfo{
			{
				DiscussAuditType: 2, // 2表示拒绝
				DiscussSn:        product.BargainSn,
				DocumentSn:       product.DocumentSn,
			},
		},
	}
}

// BuildReappealRequest 构造重新议价请求(使用新接口)
func (b *PricingRequestBuilder) BuildReappealRequest(
	product *pricing.BargainPageData,
	newPrices map[string]float64,
	reason string,
) *pricing.BatchReQuoteRequest {
	// 重新议价使用接受操作
	return &pricing.BatchReQuoteRequest{
		ConfirmInfos: []pricing.ReQuoteConfirmInfo{
			{
				DiscussAuditType: 3,
				DiscussSn:        product.BargainSn,
				DocumentSn:       product.DocumentSn,
			},
		},
	}
}

// BuildLegacyAcceptRequest 构造接受报价请求(使用旧接口)
func (b *PricingRequestBuilder) BuildLegacyAcceptRequest(product *pricing.BargainPageData) *pricing.BatchHandleCostDiscussRequest {
	confirmInfos := []pricing.ConfirmInfo{
		{
			DiscussAuditType: 1, // 1表示接受
			DiscussSn:        product.BargainSn,
			DocumentSn:       product.DocumentSn,
		},
	}
	return &pricing.BatchHandleCostDiscussRequest{
		ConfirmInfos:        &confirmInfos,
		CreateCostDiscusses: nil,
	}
}

// BuildLegacyRejectRequest 构造拒绝报价请求(使用旧接口)
func (b *PricingRequestBuilder) BuildLegacyRejectRequest(product *pricing.BargainPageData) *pricing.BatchHandleCostDiscussRequest {
	confirmInfos := []pricing.ConfirmInfo{
		{
			DiscussAuditType: 2, // 2表示拒绝
			DiscussSn:        product.BargainSn,
			DocumentSn:       product.DocumentSn,
		},
	}
	return &pricing.BatchHandleCostDiscussRequest{
		ConfirmInfos:        &confirmInfos,
		CreateCostDiscusses: nil,
	}
}
