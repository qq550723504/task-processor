package sku

import (
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/variant"
)

type SKUStrategyProcessor struct {
	variantMatcher *variant.VariantMatcher
	creator        *SKUCreator
	utils          *SKUUtils
}

func NewSKUStrategyProcessor(variantMatcher *variant.VariantMatcher) *SKUStrategyProcessor {
	return &SKUStrategyProcessor{
		variantMatcher: variantMatcher,
		creator:        NewSKUCreator(),
		utils:          NewSKUUtils(),
	}
}

func (p *SKUStrategyProcessor) BuildSingleSKU(ctx *shein.TaskContext, req shein.SKUBuildRequest) ([]product.SKU, error) {
	return p.BuildSingleSKUWithRuntime(ctx, newRuntimeInput(ctx), req)
}

func (p *SKUStrategyProcessor) BuildSingleSKUWithRuntime(ctx *shein.TaskContext, runtime *RuntimeInput, req shein.SKUBuildRequest) ([]product.SKU, error) {
	logger.GetGlobalLogger("shein/product").Info("start single-SKU build flow")

	var matchedVariant *shein.Variant
	var attributeTemplates []attribute.AttributeTemplate
	if runtime.AttributeTemplates != nil {
		attributeTemplates = runtime.AttributeTemplates.Data
	}

	primaryAttrName := p.utils.GetAttributeName(req.Strategy.PrimaryAttribute.AttrID, attributeTemplates)
	attrNameAlternatives := p.utils.GetAttributeNameAlternatives(req.Strategy.PrimaryAttribute.AttrID, attributeTemplates)
	allAttrNames := []string{}
	if primaryAttrName != "" {
		allAttrNames = append(allAttrNames, primaryAttrName)
	}
	allAttrNames = append(allAttrNames, attrNameAlternatives...)

	for _, variantItem := range req.SaleAttributeData.Variants {
		matched := false
		for _, attrName := range allAttrNames {
			for variantAttrKey, variantAttrValue := range variantItem.Attributes {
				if strings.EqualFold(variantAttrKey, attrName) && strings.EqualFold(variantAttrValue, req.PrimaryAttrValue) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			matchedVariant = &variantItem
			break
		}
	}

	if matchedVariant == nil {
		return []product.SKU{}, nil
	}

	var primaryValueID int
	for _, attrValue := range req.Strategy.PrimaryAttribute.AttrValue {
		if strings.EqualFold(attrValue.Value, req.PrimaryAttrValue) {
			primaryValueID = attrValue.ID.Int()
			break
		}
	}
	if primaryValueID <= 0 {
		return []product.SKU{}, fmt.Errorf("primary attribute value ID is invalid: %s", req.PrimaryAttrValue)
	}

	var saleAttributeList []product.SaleAttribute
	if req.Strategy.SecondaryAttribute.AttrID > 0 && req.Strategy.SecondaryAttribute.AttrID != req.Strategy.PrimaryAttribute.AttrID {
		logger.GetGlobalLogger("shein/product").Info("single-SKU scenario detected secondary attribute; leaving SKU sale attributes empty")
	}

	productInfo := runtime.FindProductInfoByASIN(matchedVariant.ASIN)
	skuItem, err := p.creator.CreateSKUWithRuntime(ctx, runtime, shein.SKUCreationParams{
		ASIN:              matchedVariant.ASIN,
		ProductInfo:       productInfo,
		WarehouseCode:     req.WarehouseCode,
		SaleAttributeList: saleAttributeList,
		Variant:           *matchedVariant,
	})
	if err != nil || skuItem == nil {
		return []product.SKU{}, err
	}

	return []product.SKU{*skuItem}, nil
}

func (p *SKUStrategyProcessor) BuildMultipleSKUs(ctx *shein.TaskContext, req shein.SKUBuildRequest) ([]product.SKU, error) {
	return p.BuildMultipleSKUsWithRuntime(ctx, newRuntimeInput(ctx), req)
}

func (p *SKUStrategyProcessor) BuildMultipleSKUsWithRuntime(ctx *shein.TaskContext, runtime *RuntimeInput, req shein.SKUBuildRequest) ([]product.SKU, error) {
	primaryMatchedVariants := req.MatchedVariants
	if len(primaryMatchedVariants) == 0 {
		primaryMatchedVariants = p.variantMatcher.FindMatchingVariants(ctx,
			req.SaleAttributeData.Variants,
			req.Strategy.PrimaryAttribute.AttrID,
			req.PrimaryAttrValue,
		)
	} else {
		logger.GetGlobalLogger("shein/product").Infof(
			"using pre-matched primary variants for sku build: primary_value=%q variants=%d",
			req.PrimaryAttrValue,
			len(primaryMatchedVariants),
		)
	}
	if len(primaryMatchedVariants) == 0 {
		return []product.SKU{}, nil
	}

	processedSecondaryValues := make(map[string]bool)
	variantInfoMap := make(map[string]sheinattr.VariantInfo)
	usedValueIDs := make(map[int]bool)
	secondaryValues := make([]string, 0, len(req.Strategy.SecondaryAttribute.AttrValue))
	for _, attr := range req.Strategy.SecondaryAttribute.AttrValue {
		if processedSecondaryValues[attr.Value] {
			continue
		}
		processedSecondaryValues[attr.Value] = true

		currentValueID := attr.ID.Int()
		if currentValueID <= 0 || usedValueIDs[currentValueID] {
			continue
		}
		usedValueIDs[currentValueID] = true
		secondaryValues = append(secondaryValues, attr.Value)
	}

	secondaryAssignments := p.variantMatcher.FindUniqueMatchesForValues(
		ctx,
		primaryMatchedVariants,
		req.Strategy.SecondaryAttribute.AttrID,
		secondaryValues,
	)
	logger.GetGlobalLogger("shein/product").Infof(
		"secondary unique assignment prepared: primary_value=%q candidate_variants=%d secondary_values=%d",
		req.PrimaryAttrValue,
		len(primaryMatchedVariants),
		len(secondaryValues),
	)
	for _, attr := range req.Strategy.SecondaryAttribute.AttrValue {
		currentValueID := attr.ID.Int()
		if currentValueID <= 0 {
			continue
		}
		secondaryMatchedVariants := secondaryAssignments[attr.Value]
		logger.GetGlobalLogger("shein/product").Infof(
			"secondary unique assignment consumed: primary_value=%q secondary_value=%q value_id=%d assigned_variants=%d",
			req.PrimaryAttrValue,
			attr.Value,
			currentValueID,
			len(secondaryMatchedVariants),
		)
		for _, variantItem := range secondaryMatchedVariants {
			variantInfoMap[variantItem.ASIN] = sheinattr.VariantInfo{
				Variant:   variantItem,
				AttrID:    req.Strategy.SecondaryAttribute.AttrID,
				ValueID:   currentValueID,
				AttrValue: attr.Value,
			}
		}
	}

	return p.buildSKUListForMultipleVariantsWithRuntime(ctx, runtime, variantInfoMap, req)
}

func (p *SKUStrategyProcessor) buildSKUListForMultipleVariantsWithRuntime(ctx *shein.TaskContext, runtime *RuntimeInput, variantInfoMap map[string]sheinattr.VariantInfo, req shein.SKUBuildRequest) ([]product.SKU, error) {
	var skuList []product.SKU
	usedAttributeValueIDs := make(map[int]bool)
	usedASINs := make(map[string]bool)
	usedSupplierSKUs := make(map[string]bool)

	for _, varInfo := range variantInfoMap {
		if usedAttributeValueIDs[varInfo.ValueID] {
			continue
		}
		if usedASINs[varInfo.Variant.ASIN] {
			logger.GetGlobalLogger("shein/product").Warnf("duplicate variant ASIN detected during SKU build, skipping ASIN=%s", varInfo.Variant.ASIN)
			continue
		}
		usedAttributeValueIDs[varInfo.ValueID] = true

		productInfo := runtime.FindProductInfoByASIN(varInfo.Variant.ASIN)
		if productInfo == nil {
			productInfo = runtime.AmazonProduct
		}

		var saleAttributeList []product.SaleAttribute
		if varInfo.AttrID != req.Strategy.PrimaryAttribute.AttrID {
			saleAttributeList = []product.SaleAttribute{{
				AttributeID:        varInfo.AttrID,
				AttributeValueID:   varInfo.ValueID,
				IsSPPSaleAttribute: false,
				PreFillSpec:        false,
			}}
		}

		skuItem, err := p.creator.CreateSKUWithRuntime(ctx, runtime, shein.SKUCreationParams{
			ASIN:              varInfo.Variant.ASIN,
			ProductInfo:       productInfo,
			WarehouseCode:     req.WarehouseCode,
			SaleAttributeList: saleAttributeList,
			Variant:           varInfo.Variant,
		})
		if err != nil || skuItem == nil {
			continue
		}
		if skuItem.SupplierSKU != "" && usedSupplierSKUs[skuItem.SupplierSKU] {
			logger.GetGlobalLogger("shein/product").Warnf(
				"duplicate supplier SKU detected during SKU build, skipping supplierSKU=%s asin=%s attrValue=%s",
				skuItem.SupplierSKU,
				varInfo.Variant.ASIN,
				varInfo.AttrValue,
			)
			continue
		}
		usedASINs[varInfo.Variant.ASIN] = true
		if skuItem.SupplierSKU != "" {
			usedSupplierSKUs[skuItem.SupplierSKU] = true
		}
		logger.GetGlobalLogger("shein/product").Infof(
			"sku build assignment: asin=%s supplier_sku=%s secondary_attr_id=%d secondary_value=%q secondary_value_id=%d",
			varInfo.Variant.ASIN,
			skuItem.SupplierSKU,
			varInfo.AttrID,
			varInfo.AttrValue,
			varInfo.ValueID,
		)
		skuList = append(skuList, *skuItem)
	}

	if len(skuList) == 0 {
		return []product.SKU{}, nil
	}
	return skuList, nil
}

func findProductInfoOrFallback(runtime *RuntimeInput, asin string) *model.Product {
	if runtime == nil {
		return nil
	}
	return runtime.FindProductInfoByASIN(asin)
}
