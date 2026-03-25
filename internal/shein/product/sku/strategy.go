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
	skuItem, err := p.creator.CreateSKU(ctx, shein.SKUCreationParams{
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
	primaryMatchedVariants := p.variantMatcher.FindMatchingVariants(ctx,
		req.SaleAttributeData.Variants,
		req.Strategy.PrimaryAttribute.AttrID,
		req.PrimaryAttrValue,
	)
	if len(primaryMatchedVariants) == 0 {
		return []product.SKU{}, nil
	}

	processedSecondaryValues := make(map[string]bool)
	variantInfoMap := make(map[string]sheinattr.VariantInfo)
	usedValueIDs := make(map[int]bool)
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

		secondaryMatchedVariants := p.variantMatcher.FindMatchingVariants(ctx,
			primaryMatchedVariants,
			req.Strategy.SecondaryAttribute.AttrID,
			attr.Value,
		)

		for _, variantItem := range secondaryMatchedVariants {
			compositeKey := fmt.Sprintf("%s:%d", variantItem.ASIN, currentValueID)
			if _, exists := variantInfoMap[compositeKey]; !exists {
				variantInfoMap[compositeKey] = sheinattr.VariantInfo{
					Variant:   variantItem,
					AttrID:    req.Strategy.SecondaryAttribute.AttrID,
					ValueID:   currentValueID,
					AttrValue: attr.Value,
				}
			}
		}
	}

	return p.buildSKUListForMultipleVariantsWithRuntime(ctx, runtime, variantInfoMap, req)
}

func (p *SKUStrategyProcessor) buildSKUListForMultipleVariantsWithRuntime(ctx *shein.TaskContext, runtime *RuntimeInput, variantInfoMap map[string]sheinattr.VariantInfo, req shein.SKUBuildRequest) ([]product.SKU, error) {
	var skuList []product.SKU
	usedAttributeValueIDs := make(map[int]bool)

	for _, varInfo := range variantInfoMap {
		if usedAttributeValueIDs[varInfo.ValueID] {
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

		skuItem, err := p.creator.CreateSKU(ctx, shein.SKUCreationParams{
			ASIN:              varInfo.Variant.ASIN,
			ProductInfo:       productInfo,
			WarehouseCode:     req.WarehouseCode,
			SaleAttributeList: saleAttributeList,
			Variant:           varInfo.Variant,
		})
		if err != nil || skuItem == nil {
			continue
		}
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
