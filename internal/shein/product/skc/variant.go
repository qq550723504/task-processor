// Package skc 提供SHEIN平台SKC变体处理功能
package skc

import (
	"fmt"
	"strings"

	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	shein "task-processor/internal/shein"
	api_attribute "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/product/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/product/sku"
	"task-processor/internal/shein/product/variant"
)

type SKCVariantProcessor struct {
	imageProcessor  *image.ImageProcessor
	attributeMapper *attribute.AttributeMapper
	skuBuilder      *sku.SKUBuilder
	runtime         *SKCRuntimeInput
	openaiClient    openaiClient.ChatCompleter
}

func NewSKCVariantProcessor(imageProcessor *image.ImageProcessor, attributeMapper *attribute.AttributeMapper, skuBuilder *sku.SKUBuilder, runtime *SKCRuntimeInput, openaiClient openaiClient.ChatCompleter) *SKCVariantProcessor {
	return &SKCVariantProcessor{
		imageProcessor:  imageProcessor,
		attributeMapper: attributeMapper,
		skuBuilder:      skuBuilder,
		runtime:         runtime,
		openaiClient:    openaiClient,
	}
}

func (p *SKCVariantProcessor) BuildSingleVariantSKC(input *SKCVariantBuildInput, ctx *shein.TaskContext, strategy sheinattr.AttributeStrategy) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	if err := input.Validate(); err != nil {
		return nil, nil, err
	}

	logger.GetGlobalLogger("shein/product").Info("start single-variant SKC build")

	variantItem := input.SaleAttributeData.Variants[0]
	var customAttributeRelations []api_attribute.CustomAttributeRelation

	mapperRuntime := &attribute.MapperRuntimeInput{
		CategoryID:         input.CategoryID,
		ProductTitle:       p.runtime.AmazonProduct.Title,
		AttributeTemplates: p.runtime.AttributeTemplates,
		AttributeAPI:       input.AttributeAPI,
	}
	mappingRelations, err := p.attributeMapper.MapAttributeValuesToSheinIDsWithRuntime(ctx, mapperRuntime, &strategy)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to map attribute values to SHEIN IDs: %w", err)
	}
	customAttributeRelations = append(customAttributeRelations, mappingRelations...)

	if len(strategy.PrimaryAttribute.AttrValue) == 0 {
		return nil, nil, fmt.Errorf("primary attribute has no values")
	}
	primaryAttrValue := strategy.PrimaryAttribute.AttrValue[0]
	if primaryAttrValue.ID.Int() <= 0 {
		return nil, nil, fmt.Errorf("primary attribute value ID is invalid: %s", primaryAttrValue.Value)
	}

	skcList, builderRelations, err := p.buildSingleVariantDirect(input, ctx, variantItem, strategy)
	if err != nil {
		return nil, nil, err
	}

	return skcList, append(customAttributeRelations, builderRelations...), nil
}

func (p *SKCVariantProcessor) BuildMultiVariantSKCList(input *SKCVariantBuildInput, ctx *shein.TaskContext, strategy sheinattr.AttributeStrategy, variantMatcher *variant.VariantMatcher) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	if err := input.Validate(); err != nil {
		return nil, nil, err
	}

	skcList := make([]product.SKC, 0, len(strategy.PrimaryAttribute.AttrValue))
	var customAttributeRelations []api_attribute.CustomAttributeRelation

	processedValues := make(map[string]bool)
	usedAttributeValueIDs := make(map[int]bool)

	mapperRuntime := &attribute.MapperRuntimeInput{
		CategoryID:         input.CategoryID,
		ProductTitle:       p.runtime.AmazonProduct.Title,
		AttributeTemplates: p.runtime.AttributeTemplates,
		AttributeAPI:       input.AttributeAPI,
	}
	mappingRelations, err := p.attributeMapper.MapAttributeValuesToSheinIDsWithRuntime(ctx, mapperRuntime, &strategy)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to map attribute values to SHEIN IDs: %w", err)
	}
	customAttributeRelations = append(customAttributeRelations, mappingRelations...)

	p.ensureVariantsHaveRequiredAttributes(input, strategy)

	skuRuntime := &sku.RuntimeInput{
		AmazonProduct:      p.runtime.AmazonProduct,
		Variants:           p.runtime.Variants,
		AttributeTemplates: p.runtime.AttributeTemplates,
	}

	for i := 0; i < len(strategy.PrimaryAttribute.AttrValue); i++ {
		attrValue := &strategy.PrimaryAttribute.AttrValue[i]
		if processedValues[attrValue.Value] {
			continue
		}
		attrValueID := attrValue.ID.Int()
		if usedAttributeValueIDs[attrValueID] || attrValueID <= 0 {
			continue
		}
		processedValues[attrValue.Value] = true
		usedAttributeValueIDs[attrValueID] = true

		matchedVariants := variantMatcher.FindMatchingVariants(ctx, input.SaleAttributeData.Variants, strategy.PrimaryAttribute.AttrID, attrValue.Value)
		if len(matchedVariants) == 0 {
			continue
		}

		imageHandler := NewSKCImageHandler(p.imageProcessor, p.runtime)
		imagesToUse, err := imageHandler.GetVariantSpecificImages(matchedVariants[0])
		if err != nil || len(imagesToUse) == 0 {
			imagesToUse = p.runtime.AmazonProduct.Images
		}

		imageInfo, err := p.imageProcessor.BuildImageInfo(ctx, imagesToUse)
		if err != nil {
			imageInfo = product.ImageInfo{}
		}

		skuBuildReq := shein.SKUBuildRequest{
			SaleAttributeData: input.SaleAttributeData,
			Strategy:          strategy,
			PrimaryAttrValue:  attrValue.Value,
			WarehouseCode:     input.WarehouseCode,
		}
		skuList, err := p.skuBuilder.BuildSKUListWithRuntime(ctx, skuRuntime, skuBuildReq)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build SKU list: %w", err)
		}
		if len(skuList) == 0 {
			continue
		}

		translationHandler := NewSKCTranslationHandler(p.runtime, p.openaiClient)
		skc := translationHandler.CreateSKC(ctx, shein.SKCCreationParams{
			AttributeID:      strategy.PrimaryAttribute.AttrID,
			AttributeValueID: attrValueID,
			SKUS:             skuList,
			ImageInfo:        imageInfo,
			SupplierCode:     "",
			Sort:             i + 1,
		})

		p.autoFixMultiPieceSKUImages(&skc, &imageInfo)
		skcList = append(skcList, skc)
	}

	return skcList, customAttributeRelations, nil
}

func (p *SKCVariantProcessor) buildSingleVariantDirect(input *SKCVariantBuildInput, ctx *shein.TaskContext, variantItem shein.Variant, strategy sheinattr.AttributeStrategy) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	var customAttributeRelations []api_attribute.CustomAttributeRelation

	imageInfo, err := p.imageProcessor.BuildImageInfo(ctx, p.runtime.AmazonProduct.Images)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build image info: %w", err)
	}

	if len(strategy.PrimaryAttribute.AttrValue) == 0 {
		return nil, nil, fmt.Errorf("primary attribute has no values")
	}
	primaryAttrValue := strategy.PrimaryAttribute.AttrValue[0]
	if primaryAttrValue.ID.Int() <= 0 {
		return nil, nil, fmt.Errorf("primary attribute value ID is invalid: %s (ID: %d)", primaryAttrValue.Value, primaryAttrValue.ID.Int())
	}

	skuRuntime := &sku.RuntimeInput{
		AmazonProduct:      p.runtime.AmazonProduct,
		Variants:           p.runtime.Variants,
		AttributeTemplates: p.runtime.AttributeTemplates,
	}
	skuList, err := p.skuBuilder.BuildSKUListForSingleVariantWithRuntime(ctx, skuRuntime, variantItem, strategy, input.WarehouseCode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build SKU list: %w", err)
	}
	if len(skuList) == 0 {
		return nil, nil, fmt.Errorf("failed to build valid SKUs for single variant")
	}

	hasSecondaryAttribute := strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0
	if !hasSecondaryAttribute && len(skuList) > 1 {
		logger.GetGlobalLogger("shein/product").Warnf("single-variant build produced %d SKUs without a secondary attribute; keeping the first only", len(skuList))
		skuList = skuList[:1]
	}

	translationHandler := NewSKCTranslationHandler(p.runtime, p.openaiClient)
	skc := translationHandler.CreateSKC(ctx, shein.SKCCreationParams{
		AttributeID:      strategy.PrimaryAttribute.AttrID,
		AttributeValueID: primaryAttrValue.ID.Int(),
		SKUS:             skuList,
		ImageInfo:        imageInfo,
		SupplierCode:     "",
		Sort:             1,
	})

	p.autoFixMultiPieceSKUImages(&skc, &imageInfo)
	return []product.SKC{skc}, customAttributeRelations, nil
}

func (p *SKCVariantProcessor) ensureVariantsHaveRequiredAttributes(input *SKCVariantBuildInput, strategy sheinattr.AttributeStrategy) {
	primaryAttrName := p.getAttributeNameForVariant(strategy.PrimaryAttribute.AttrID)
	secondaryAttrName := ""
	if strategy.SecondaryAttribute.AttrID > 0 {
		secondaryAttrName = p.getAttributeNameForVariant(strategy.SecondaryAttribute.AttrID)
	}

	for i := range input.SaleAttributeData.Variants {
		variantItem := &input.SaleAttributeData.Variants[i]
		if !p.variantHasAttribute(variantItem, primaryAttrName) && len(strategy.PrimaryAttribute.AttrValue) > 0 {
			if variantItem.Attributes == nil {
				variantItem.Attributes = map[string]string{}
			}
			variantItem.Attributes[primaryAttrName] = strategy.PrimaryAttribute.AttrValue[0].Value
		}
		if secondaryAttrName != "" && !p.variantHasAttribute(variantItem, secondaryAttrName) {
			logger.GetGlobalLogger("shein/product").Warnf("variant ASIN=%s is missing secondary attribute %s", variantItem.ASIN, secondaryAttrName)
		}
	}
}

func (p *SKCVariantProcessor) getAttributeNameForVariant(attrID int) string {
	if p.runtime != nil && p.runtime.AttributeTemplates != nil {
		for _, data := range p.runtime.AttributeTemplates.Data {
			for _, attrInfo := range data.AttributeInfos {
				if attrInfo.AttributeID == attrID {
					if attrInfo.AttributeNameEn != "" {
						return attrInfo.AttributeNameEn
					}
					if attrInfo.AttributeName != "" {
						return attrInfo.AttributeName
					}
				}
			}
		}
	}

	switch attrID {
	case 27:
		return "Color"
	case 87:
		return "Size"
	default:
		return fmt.Sprintf("attr_%d", attrID)
	}
}

func (p *SKCVariantProcessor) variantHasAttribute(variantItem *shein.Variant, attrName string) bool {
	if variantItem.Attributes == nil {
		return false
	}
	if _, exists := variantItem.Attributes[attrName]; exists {
		return true
	}

	attrNameLower := strings.ToLower(attrName)
	for key := range variantItem.Attributes {
		if strings.ToLower(key) == attrNameLower {
			return true
		}
	}
	return false
}

func (p *SKCVariantProcessor) autoFixMultiPieceSKUImages(skc *product.SKC, skcImageInfo *product.ImageInfo) {
	if skc == nil || len(skc.SKUS) == 0 {
		return
	}

	fixer := sku.NewSKUImageAutoFixer()
	fixedCount := 0
	for i := range skc.SKUS {
		skuItem := &skc.SKUS[i]
		if !fixer.IsMultiPieceSKU(skuItem) {
			continue
		}
		if skuItem.ImageInfo != nil && len(skuItem.ImageInfo.ImageInfoList) > 0 {
			fixer.AutoFixSKUImageSorting(skuItem)
			continue
		}
		if skcImageInfo == nil || len(skcImageInfo.ImageInfoList) == 0 {
			logger.GetGlobalLogger("shein/product").Warnf("multipart SKU %s is missing images, and no SKC image is available to copy", skuItem.SupplierSKU)
			continue
		}

		firstImage := skcImageInfo.ImageInfoList[0]
		skuItem.ImageInfo = &product.ImageInfo{
			ImageGroupCode: nil,
			ImageInfoList: []product.ImageDetail{{
				ImageType:             firstImage.ImageType,
				ImageSort:             1,
				ImageURL:              firstImage.ImageURL,
				ImageItemID:           firstImage.ImageItemID,
				SizeImgFlag:           firstImage.SizeImgFlag,
				TransformCVSizeImage:  firstImage.TransformCVSizeImage,
				AISStatus:             firstImage.AISStatus,
				PSTypes:               firstImage.PSTypes,
				MarketingMainImage:    false,
				CommodityCategoryFlag: firstImage.CommodityCategoryFlag,
			}},
			OriginalImageInfoList: &[]any{},
		}
		fixedCount++
	}

	if fixedCount > 0 {
		logger.GetGlobalLogger("shein/product").Infof("fixed images for %d multipart SKUs", fixedCount)
	}
}
