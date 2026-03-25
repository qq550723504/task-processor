package skc

import (
	"fmt"

	"task-processor/internal/core/logger"
	"task-processor/internal/infra/clients/management"
	openaiClient "task-processor/internal/infra/clients/openai"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/category"
	"task-processor/internal/shein/product/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
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
	baseStrategy := strategyHandler.DetermineAttributeStrategy(input.SaleAttributeOutput.Result, config, input.AttributeTemplates)
	strategy, isAdapted, adaptationReasons := b.adaptStrategy(baseStrategy, input.ProductData.CategoryID, input.ManagementClient)

	if isAdapted {
		logger.GetGlobalLogger("shein/product").Infof("SKC strategy adapted: reasons=%d", len(adaptationReasons))
	}

	validator := NewSKCValidationUtils(ctx)
	if err := validator.ValidateAttributeStrategy(strategy, input.SaleAttributeOutput.Result); err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("strategy validation warning: %v", err)
	}

	processor := NewSKCVariantProcessor(b.imageProcessor, b.attributeMapper, b.skuBuilder, input.Runtime, b.openaiClient)
	if input.SaleAttributeOutput.VariantCount == 1 {
		skcList, relations, err := processor.BuildSingleVariantSKC(input.VariantBuild, ctx, strategy)
		if err != nil {
			return nil, err
		}
		return newSKCBuildOutput(skcList, relations), nil
	}

	skcList, relations, err := processor.BuildMultiVariantSKCList(input.VariantBuild, ctx, strategy, b.variantMatcher)
	if err != nil {
		return nil, err
	}
	return newSKCBuildOutput(skcList, relations), nil
}

func (b *SKCBuilder) adaptStrategy(strategy sheinattr.AttributeStrategy, categoryID int, managementClient *management.ClientManager) (sheinattr.AttributeStrategy, bool, []string) {
	restriction, hasRestriction := b.getCategoryRestrictions(managementClient)[categoryID]
	if !hasRestriction {
		return strategy, false, nil
	}

	var adaptationReasons []string
	isAdapted := false
	adaptedStrategy := strategy

	if strategy.PrimaryAttribute.AttrID == restriction.ForbiddenPrimarySpec {
		defaultColorAttr := sheinattr.ResultAttribute{
			AttrID: restriction.DefaultPrimarySpec,
			AttrValue: []sheinattr.AttributeValue{{
				ID:    -1,
				Value: "Multi-Color",
			}},
		}
		adaptedStrategy.SecondaryAttribute = strategy.PrimaryAttribute
		adaptedStrategy.PrimaryAttribute = defaultColorAttr
		adaptationReasons = append(adaptationReasons, fmt.Sprintf("primary spec replaced: %d -> %d", strategy.PrimaryAttribute.AttrID, restriction.DefaultPrimarySpec))
		isAdapted = true
	}

	if adaptedStrategy.PrimaryAttribute.AttrID > 0 && adaptedStrategy.SecondaryAttribute.AttrID > 0 && adaptedStrategy.PrimaryAttribute.AttrID == adaptedStrategy.SecondaryAttribute.AttrID {
		adaptedStrategy.SecondaryAttribute = sheinattr.ResultAttribute{AttrID: -1}
		adaptationReasons = append(adaptationReasons, "secondary spec cleared to avoid conflict with primary spec")
		isAdapted = true
	}

	return adaptedStrategy, isAdapted, adaptationReasons
}

func (b *SKCBuilder) getCategoryRestrictions(managementClient *management.ClientManager) map[int]category.CategoryRestriction {
	if managementClient == nil {
		logger.GetGlobalLogger("shein/product").Warn("management client is nil, returning empty category restrictions")
		return make(map[int]category.CategoryRestriction)
	}

	categoryRestrictions := make(map[int]category.CategoryRestriction)
	restrictions, err := managementClient.GetCategoryRestrictionCache().GetConfirmedListByPlatform("Shein")
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("get category restrictions failed: %v", err)
		return categoryRestrictions
	}

	for _, restriction := range restrictions {
		categoryRestrictions[restriction.CategoryId] = category.CategoryRestriction{
			CategoryID:           restriction.CategoryId,
			ForbiddenPrimarySpec: restriction.ForbiddenAttributeId,
			DefaultPrimarySpec:   restriction.DefaultAttributeId,
			PlatformName:         restriction.PlatformName,
		}
	}
	return categoryRestrictions
}
