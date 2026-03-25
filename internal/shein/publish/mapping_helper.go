// Package publish 提供产品导入映射请求的构建辅助函数
package publish

import (
	"fmt"

	management_api "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	"task-processor/internal/shein/productdata"
	"task-processor/internal/shein/validation"
)

// buildMappingReq 构建产品导入映射请求，供 result.go 和 variant_success.go 共用。
// asin 为来源平台产品ID，supplierSKU 为供应商SKU，status 为映射状态。
func buildMappingReq(input *MappingRequestInput, asin, supplierSKU string, status model.TaskStatus) *management_api.ProductImportMappingCreateReqDTO {
	s := status.Int16()
	req := &management_api.ProductImportMappingCreateReqDTO{
		TenantID:     input.Task.TenantID,
		ImportTaskId: input.Task.ID,
		StoreId:      input.Task.StoreID,
		Platform:     input.Task.Platform,
		Region:       input.Task.Region,
		ProductId:    asin,
		Status:       &s,
	}

	if supplierSKU != "" {
		req.Sku = &supplierSKU
	}

	// 成本价
	variant := productdata.GetVariantByAsinFromVariants(input.Variants, asin)
	if variant == nil {
		variant = productdata.GetVariantByAsinFromVariants(input.UnfilteredVariants, asin)
	}
	if variant != nil && input.StoreInfo != nil {
		costPrice := validation.GetProductPrice(variant, input.StoreInfo.PriceType)
		req.CostPrice = &costPrice
	}

	// 父产品ID
	if input.AmazonProduct != nil && input.AmazonProduct.ParentAsin != "" {
		req.ParentProductId = &input.AmazonProduct.ParentAsin
	}

	// 平台父产品ID
	if input.ProductData != nil && input.ProductData.SPUName != "" {
		req.PlatformParentProductId = &input.ProductData.SPUName
	}

	// 筛选规则
	if input.FilterRule != nil {
		req.FilterRuleId = &input.FilterRule.ID
		filterRuleRange := buildFilterRuleRange(input.FilterRule)
		if filterRuleRange != "" {
			req.FilterRuleRange = &filterRuleRange
		}
	}

	// 利润规则
	if input.ProfitRule != nil {
		req.ProfitRuleId = &input.ProfitRule.ID
		salePriceMultiplier := fmt.Sprintf("%.2f", input.ProfitRule.SalePriceMultiplier)
		req.SalePriceMultiplier = &salePriceMultiplier
		if input.ProfitRule.DiscountPriceMultiplier > 0 {
			discountPriceMultiplier := fmt.Sprintf("%.2f", input.ProfitRule.DiscountPriceMultiplier)
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
