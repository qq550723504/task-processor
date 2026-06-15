package sale

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"
)

type SaleAttributeVariantFilter struct{}

func NewSaleAttributeVariantFilter() *SaleAttributeVariantFilter {
	return &SaleAttributeVariantFilter{}
}

func (f *SaleAttributeVariantFilter) FilterVariantsByRules(ctx *sheinctx.TaskContext) {
	variants := ctx.FilteredVariants()
	if variants == nil {
		return
	}

	filteredVariants := make([]model.Product, 0, len(variants))
	filteredOutCount := 0
	for _, variant := range variants {
		filterInfo := ctx.GetVariantFilterInfo(variant.Asin)
		if filterInfo != nil && filterInfo.FilteredOut {
			filteredOutCount++
			continue
		}
		filteredVariants = append(filteredVariants, variant)
	}

	ctx.SetVariants(filteredVariants)
	logger.GetGlobalLogger("shein/product").Debugf("filtered variants before sale attribute generation: removed=%d kept=%d", filteredOutCount, len(filteredVariants))
}

func (f *SaleAttributeVariantFilter) FilterVariantsByRulesAfterGeneration(ctx *sheinctx.TaskContext, saleAttributeData *sheinattr.ResultSaleAttribute) {
	if saleAttributeData == nil {
		return
	}

	filteredVariants := make([]sheinattr.Variant, 0, len(saleAttributeData.Variants))
	filteredOutCount := 0
	for _, variant := range saleAttributeData.Variants {
		filterInfo := ctx.GetVariantFilterInfo(variant.ASIN)
		if filterInfo != nil && filterInfo.FilteredOut {
			filteredOutCount++
			continue
		}
		filteredVariants = append(filteredVariants, variant)
	}

	saleAttributeData.Variants = filteredVariants
	logger.GetGlobalLogger("shein/product").Debugf("filtered variants after sale attribute generation: removed=%d kept=%d", filteredOutCount, len(filteredVariants))
}
