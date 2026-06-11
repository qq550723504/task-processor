package skc

import (
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/product/sku"
	"task-processor/internal/shein/product/variant"
)

type SKCBuilder struct {
	imageProcessor  *image.ImageProcessor
	attributeMapper *attribute.AttributeMapper
	variantMatcher  *variant.VariantMatcher
	skuBuilder      *sku.SKUBuilder
	openaiClient    openaiClient.ChatCompleter
}

func NewSKCBuilder(imageProcessor *image.ImageProcessor, attributeMapper *attribute.AttributeMapper, variantMatcher *variant.VariantMatcher, skuBuilder *sku.SKUBuilder, openaiClient openaiClient.ChatCompleter) *SKCBuilder {
	return &SKCBuilder{
		imageProcessor:  imageProcessor,
		attributeMapper: attributeMapper,
		variantMatcher:  variantMatcher,
		skuBuilder:      skuBuilder,
		openaiClient:    openaiClient,
	}
}

func (b *SKCBuilder) BuildSKCListWithSpecAdaptation(input *SKCBuildInput, ctx *shein.TaskContext, strategyHandler *AttributeStrategyHandler) (*SKCBuildOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	logger.GetGlobalLogger("shein/product").Info("start SKC build flow")

	config := strategyHandler.GetDynamicAttributePriorityConfig(input.AttributeTemplates)
	saleSpecData := input.SaleAttributeOutput.Result
	strategy, adaptedSaleSpec, strategySource, err := BuildStrategyFromSelection(ctx, &saleSpecData, input.AttributeTemplates)
	if err != nil {
		strategy = strategyHandler.DetermineAttributeStrategy(input.SaleAttributeOutput.Result, config, input.AttributeTemplates)
		strategySource = "legacy"
	} else {
		saleSpecData = adaptedSaleSpec
	}
	logger.GetGlobalLogger("shein/product").Infof(
		"SKC strategy selected: source=%s primary=%d secondary=%d type=%s",
		strategySource,
		strategy.PrimaryAttribute.AttrID,
		strategy.SecondaryAttribute.AttrID,
		strategy.StrategyType,
	)

	validator := NewSKCValidationUtils()
	validationInput := *input.Validation
	variantBuildInput := *input.VariantBuild
	if strategySource == "selection" {
		validationInput.StrategyData = saleSpecData
		variantBuildInput.SaleAttributeData = saleSpecData
	}
	if err := validator.ValidateAttributeStrategy(&validationInput, strategy); err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("strategy validation warning: %v", err)
	}

	processor := NewSKCVariantProcessor(b.imageProcessor, b.attributeMapper, b.skuBuilder, input.Runtime, b.openaiClient)
	if input.SaleAttributeOutput.VariantCount == 1 {
		skcList, relations, err := processor.BuildSingleVariantSKC(&variantBuildInput, ctx, strategy)
		if err != nil {
			return nil, err
		}
		return newSKCBuildOutput(skcList, relations), nil
	}

	skcList, relations, err := processor.BuildMultiVariantSKCList(&variantBuildInput, ctx, strategy, b.variantMatcher)
	if err != nil {
		return nil, err
	}
	return newSKCBuildOutput(skcList, relations), nil
}
