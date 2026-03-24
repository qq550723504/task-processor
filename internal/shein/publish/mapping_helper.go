// Package publish 提供产品导入映射请求的构建辅助函数
package publish

import (
	"fmt"

	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/productdata"
	"task-processor/internal/shein/validation"
)

// buildMappingReq 构建产品导入映射请求，供 result.go 和 variant_success.go 共用。
// asin 为来源平台产品ID，supplierSKU 为供应商SKU，status 为映射状态。
func buildMappingReq(ctx *shein.TaskContext, asin, supplierSKU string, status model.TaskStatus) *management_api.ProductImportMappingCreateReqDTO {
	s := status.Int16()
	req := &management_api.ProductImportMappingCreateReqDTO{
		TenantID:     ctx.Task.TenantID,
		ImportTaskId: ctx.Task.ID,
		StoreId:      ctx.Task.StoreID,
		Platform:     ctx.Task.Platform,
		Region:       ctx.Task.Region,
		ProductId:    asin,
		Status:       &s,
	}

	if supplierSKU != "" {
		req.Sku = &supplierSKU
	}

	// 成本价
	variant := productdata.GetVariantByAsinFromVariants(ctx.Variants, asin)
	if variant == nil {
		variant = productdata.GetVariantByAsinFromVariants(ctx.UnFilteredVariants, asin)
	}
	if variant != nil && ctx.StoreInfo != nil {
		costPrice := validation.GetProductPrice(variant, ctx.StoreInfo.PriceType)
		req.CostPrice = &costPrice
	}

	// 父产品ID
	if ctx.AmazonProduct != nil && ctx.AmazonProduct.ParentAsin != "" {
		req.ParentProductId = &ctx.AmazonProduct.ParentAsin
	}

	// 平台父产品ID
	if ctx.ProductData != nil && ctx.ProductData.SPUName != "" {
		req.PlatformParentProductId = &ctx.ProductData.SPUName
	}

	// 筛选规则
	if ctx.FilterRule != nil {
		req.FilterRuleId = &ctx.FilterRule.ID
		filterRuleRange := buildFilterRuleRange(ctx.FilterRule)
		if filterRuleRange != "" {
			req.FilterRuleRange = &filterRuleRange
		}
	}

	// 利润规则
	if ctx.ProfitRule != nil {
		req.ProfitRuleId = &ctx.ProfitRule.ID
		salePriceMultiplier := fmt.Sprintf("%.2f", ctx.ProfitRule.SalePriceMultiplier)
		req.SalePriceMultiplier = &salePriceMultiplier
		if ctx.ProfitRule.DiscountPriceMultiplier > 0 {
			discountPriceMultiplier := fmt.Sprintf("%.2f", ctx.ProfitRule.DiscountPriceMultiplier)
			req.DiscountPriceMultiplier = &discountPriceMultiplier
		}
	}

	return req
}

// buildFilterRuleRange 将筛选规则的价格范围格式化为字符串。
func buildFilterRuleRange(rule *management_api.FilterRuleRespDTO) string {
	if rule.PriceMin != nil && rule.PriceMax != nil {
		return fmt.Sprintf("%.2f-%.2f", *rule.PriceMin, *rule.PriceMax)
	}
	if rule.PriceMin != nil {
		return fmt.Sprintf("%.2f-", *rule.PriceMin)
	}
	if rule.PriceMax != nil {
		return fmt.Sprintf("-%.2f", *rule.PriceMax)
	}
	return ""
}
