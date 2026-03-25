package sku

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/product"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/variant"
)

type SKUBuilder struct {
	variantMatcher    *variant.VariantMatcher
	strategyProcessor *SKUStrategyProcessor
	creator           *SKUCreator
}

func NewSKUBuilder(variantMatcher *variant.VariantMatcher) *SKUBuilder {
	return &SKUBuilder{
		variantMatcher:    variantMatcher,
		strategyProcessor: NewSKUStrategyProcessor(variantMatcher),
		creator:           NewSKUCreator(),
	}
}

func (b *SKUBuilder) BuildSKUListWithStrategy(ctx *shein.TaskContext, req shein.SKUBuildRequest) ([]product.SKU, error) {
	return b.BuildSKUListWithRuntime(ctx, newRuntimeInput(ctx), req)
}

func (b *SKUBuilder) BuildSKUListWithRuntime(ctx *shein.TaskContext, runtime *RuntimeInput, req shein.SKUBuildRequest) ([]product.SKU, error) {
	if err := runtime.Validate(); err != nil {
		return nil, err
	}

	logger.GetGlobalLogger("shein/product").Infof("start SKU build flow")
	logger.GetGlobalLogger("shein/product").Infof("primary attribute ID=%d secondary attribute ID=%d primary value=%s variants=%d",
		req.Strategy.PrimaryAttribute.AttrID,
		req.Strategy.SecondaryAttribute.AttrID,
		req.PrimaryAttrValue,
		len(req.SaleAttributeData.Variants),
	)

	if req.Strategy.SecondaryAttribute.AttrID > 0 && req.Strategy.SecondaryAttribute.AttrID == req.Strategy.PrimaryAttribute.AttrID {
		logger.GetGlobalLogger("shein/product").Warnf("secondary attribute ID %d conflicts with primary attribute ID %d; falling back to single-SKU mode",
			req.Strategy.SecondaryAttribute.AttrID, req.Strategy.PrimaryAttribute.AttrID)
		return b.strategyProcessor.BuildSingleSKUWithRuntime(ctx, runtime, req)
	}

	if req.Strategy.SecondaryAttribute.AttrID <= 0 {
		logger.GetGlobalLogger("shein/product").Info("no secondary attribute; using single-SKU mode")
		return b.strategyProcessor.BuildSingleSKUWithRuntime(ctx, runtime, req)
	}

	logger.GetGlobalLogger("shein/product").Infof("secondary attribute values=%d; using multi-SKU mode", len(req.Strategy.SecondaryAttribute.AttrValue))
	return b.strategyProcessor.BuildMultipleSKUsWithRuntime(ctx, runtime, req)
}

func (b *SKUBuilder) BuildSKUListForSingleVariant(ctx *shein.TaskContext, variant shein.Variant, strategy sheinattr.AttributeStrategy) ([]product.SKU, error) {
	warehouseCode := ""
	if ctx.Warehouses != nil && len(ctx.Warehouses.Data) > 0 {
		warehouseCode = ctx.Warehouses.Data[0].WarehouseCode
	}
	return b.BuildSKUListForSingleVariantWithRuntime(ctx, newRuntimeInput(ctx), variant, strategy, warehouseCode)
}

func (b *SKUBuilder) BuildSKUListForSingleVariantWithRuntime(ctx *shein.TaskContext, runtime *RuntimeInput, variant shein.Variant, strategy sheinattr.AttributeStrategy, warehouseCode string) ([]product.SKU, error) {
	if err := runtime.Validate(); err != nil {
		return nil, err
	}
	if warehouseCode == "" {
		return nil, fmt.Errorf("warehouse code is not initialized")
	}

	logger.GetGlobalLogger("shein/product").Infof("build SKU list for single variant: ASIN=%s", variant.ASIN)

	var saleAttributeList []product.SaleAttribute
	if strategy.SecondaryAttribute.AttrID != 0 {
		saleAttributeList = b.creator.BuildSaleAttributeListForSingleVariantWithRuntime(runtime, variant, strategy)
	}

	productInfo := runtime.FindProductInfoByASIN(variant.ASIN)
	if productInfo == nil {
		return nil, fmt.Errorf("product info not found for ASIN %s", variant.ASIN)
	}

	skuItem, err := b.creator.CreateSKU(ctx, shein.SKUCreationParams{
		ASIN:              variant.ASIN,
		ProductInfo:       productInfo,
		WarehouseCode:     warehouseCode,
		SaleAttributeList: saleAttributeList,
		Variant:           variant,
	})
	if err != nil {
		return nil, err
	}
	if skuItem == nil {
		return []product.SKU{}, nil
	}

	logger.GetGlobalLogger("shein/product").Infof("built single-variant SKU with %d sale attributes", len(saleAttributeList))
	return []product.SKU{*skuItem}, nil
}

func findProductInfo(runtime *RuntimeInput, asin string) *model.Product {
	if runtime == nil {
		return nil
	}
	return runtime.FindProductInfoByASIN(asin)
}
