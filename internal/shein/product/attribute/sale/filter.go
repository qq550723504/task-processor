// Package sale 提供SHEIN平台销售属性的变体过滤功能
package sale

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"

)

// SaleAttributeVariantFilter 销售属性变体过滤器
type SaleAttributeVariantFilter struct{}

// NewSaleAttributeVariantFilter 创建变体过滤器实例
func NewSaleAttributeVariantFilter() *SaleAttributeVariantFilter {
	return &SaleAttributeVariantFilter{}
}

// FilterVariantsByRules 在生成销售属性之前过滤变体
func (f *SaleAttributeVariantFilter) FilterVariantsByRules(ctx *sheinctx.TaskContext) {
	if ctx.Variants == nil {
		return
	}
	filteredVariants := make([]model.Product, 0, len(*ctx.Variants))
	filteredOutCount := 0
	for _, variant := range *ctx.Variants {
		filterInfo := ctx.GetVariantFilterInfo(variant.Asin)
		if filterInfo != nil && filterInfo.FilteredOut {
			logger.GetGlobalLogger("shein/product").Infof("变体ASIN %s 已被筛选规则排除: %s，将被排除\n", variant.Asin, filterInfo.FilterReason)
			filteredOutCount++
		} else {
			filteredVariants = append(filteredVariants, variant)
		}
	}
	*ctx.Variants = filteredVariants
	logger.GetGlobalLogger("shein/product").Infof("在生成销售属性之前，已过滤掉 %d 个不符合筛选规则的变体，剩余 %d 个变体\n", filteredOutCount, len(filteredVariants))
}

// FilterVariantsByRulesAfterGeneration 在生成销售属性之后过滤变体
func (f *SaleAttributeVariantFilter) FilterVariantsByRulesAfterGeneration(ctx *sheinctx.TaskContext, saleAttributeData *sheinattr.ResultSaleAttribute) {
	if saleAttributeData == nil {
		return
	}
	filteredVariants := make([]sheinattr.Variant, 0, len(saleAttributeData.Variants))
	filteredOutCount := 0
	for _, variant := range saleAttributeData.Variants {
		filterInfo := ctx.GetVariantFilterInfo(variant.ASIN)
		if filterInfo != nil && filterInfo.FilteredOut {
			logger.GetGlobalLogger("shein/product").Infof("变体ASIN %s 已被筛选规则排除: %s，将被排除\n", variant.ASIN, filterInfo.FilterReason)
			filteredOutCount++
			continue
		}
		filteredVariants = append(filteredVariants, variant)
	}
	saleAttributeData.Variants = filteredVariants
	logger.GetGlobalLogger("shein/product").Infof("在生成销售属性之后，已过滤掉 %d 个不符合筛选规则的变体，剩余 %d 个变体\n", filteredOutCount, len(filteredVariants))
}


