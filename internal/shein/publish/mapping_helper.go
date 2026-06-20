// Package publish 提供产品导入映射请求的构建辅助函数
package publish

import (
	"strconv"
	"strings"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/shein/productdata"
	"task-processor/internal/shein/validation"
)

// buildMappingReq 构建产品导入映射请求，供 result.go 和 variant_success.go 共用。
// asin 为来源平台产品ID，supplierSKU 为供应商SKU，status 为映射状态。
func buildMappingReq(input *MappingRequestInput, asin, supplierSKU string, status model.TaskStatus) *listingruntime.ProductImportMappingUpsert {
	s := status.Int16()
	req := &listingruntime.ProductImportMappingUpsert{
		TenantID:     input.Task.TenantID,
		ImportTaskID: input.Task.ID,
		StoreID:      input.Task.StoreID,
		Platform:     input.Task.Platform,
		Region:       input.Task.Region,
		ProductID:    asin,
		Status:       &s,
	}

	if supplierSKU != "" {
		req.SKU = &supplierSKU
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
		req.ParentProductID = &input.AmazonProduct.ParentAsin
	}

	// 平台父产品ID
	if input.ProductData != nil && input.ProductData.SPUName != "" {
		req.PlatformParentProductID = &input.ProductData.SPUName
	}

	// 筛选规则
	if input.FilterRule != nil {
		req.FilterRuleID = &input.FilterRule.ID
		filterRuleRange := buildFilterRuleRange(input.FilterRule)
		if filterRuleRange != "" {
			req.FilterRuleRange = &filterRuleRange
		}
	}

	// 利润规则
	if input.ProfitRule != nil {
		req.ProfitRuleID = &input.ProfitRule.ID
		salePriceMultiplier := input.ProfitRule.SalePriceMultiplier
		req.SalePriceMultiplier = &salePriceMultiplier
		if input.ProfitRule.DiscountPriceMultiplier > 0 {
			discountPriceMultiplier := input.ProfitRule.DiscountPriceMultiplier
			req.DiscountPriceMultiplier = &discountPriceMultiplier
		}
	}

	return req
}

// buildFilterRuleRange 将筛选规则的价格范围格式化为字符串。
func buildFilterRuleRange(rule *listingruntime.FilterRule) string {
	if rule.PriceMin != nil && rule.PriceMax != nil {
		return strings.TrimSpace(formatRange(*rule.PriceMin, *rule.PriceMax, true, true))
	}
	if rule.PriceMin != nil {
		return strings.TrimSpace(formatRange(*rule.PriceMin, 0, true, false))
	}
	if rule.PriceMax != nil {
		return strings.TrimSpace(formatRange(0, *rule.PriceMax, false, true))
	}
	return ""
}

func formatRange(min float64, max float64, hasMin bool, hasMax bool) string {
	switch {
	case hasMin && hasMax:
		return strconvFloat(min) + "-" + strconvFloat(max)
	case hasMin:
		return strconvFloat(min) + "-"
	case hasMax:
		return "-" + strconvFloat(max)
	default:
		return ""
	}
}

func strconvFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmtFloat(value), "0"), ".")
}

func fmtFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}
