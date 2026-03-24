// Package skc 提供SHEIN平台SKC构建核心功能
package skc

import (
	"fmt"
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
	sheinattr "task-processor/internal/shein/product/attribute"
	api_attribute "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/category"
	"task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/product/sku"
	"task-processor/internal/shein/product/variant"
)

// SKCBuilder SKC构建器
type SKCBuilder struct {
	imageProcessor  *image.ImageProcessor
	attributeMapper *attribute.AttributeMapper
	variantMatcher  *variant.VariantMatcher
	skuBuilder      *sku.SKUBuilder
	taskContext     *shein.TaskContext
	openaiClient    openaiClient.ChatCompleter
}

// NewSKCBuilder 创建新的SKC构建器
func NewSKCBuilder(imageProcessor *image.ImageProcessor, attributeMapper *attribute.AttributeMapper, variantMatcher *variant.VariantMatcher, skuBuilder *sku.SKUBuilder, openaiClient openaiClient.ChatCompleter) *SKCBuilder {
	return &SKCBuilder{
		imageProcessor:  imageProcessor,
		attributeMapper: attributeMapper,
		variantMatcher:  variantMatcher,
		skuBuilder:      skuBuilder,
		openaiClient:    openaiClient,
	}
}

// BuildSKCListWithSpecAdaptation 构建SKC列表并进行规格适配
func (b *SKCBuilder) BuildSKCListWithSpecAdaptation(ctx *shein.TaskContext, strategyHandler *AttributeStrategyHandler) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	logger.GetGlobalLogger("shein/product").Infof("🏗️ === 开始SKC构建流程 ===")

	// 设置任务上下文
	b.taskContext = ctx
	logger.GetGlobalLogger("shein/product").Infof("✅ 任务上下文设置完成")

	logger.GetGlobalLogger("shein/product").Infof("📊 获取动态属性优先级配置...")
	config := strategyHandler.GetDynamicAttributePriorityConfig(ctx.AttributeTemplates)
	logger.GetGlobalLogger("shein/product").Infof("✅ 动态属性优先级配置获取完成")

	logger.GetGlobalLogger("shein/product").Infof("🎯 确定属性策略...")
	baseStrategy := strategyHandler.DetermineAttributeStrategy(*ctx.SaleSpecResult, config, ctx.AttributeTemplates)

	// 适配策略
	logger.GetGlobalLogger("shein/product").Infof("🔄 开始策略适配...")
	strategy, isAdapted, adaptationReasons := b.adaptStrategy(baseStrategy, ctx.ProductData.CategoryID)

	if isAdapted {
		logger.GetGlobalLogger("shein/product").Infof("✅ 策略适配完成")
		for i, reason := range adaptationReasons {
			logger.GetGlobalLogger("shein/product").Infof("  适配原因%d: %s", i+1, reason)
		}
		logger.GetGlobalLogger("shein/product").Infof("适配后策略: 主规格=%s(%d), 次规格=%s(%d)",
			b.getAttributeNameSafe(strategy.PrimaryAttribute.AttrID), strategy.PrimaryAttribute.AttrID,
			b.getAttributeNameSafe(strategy.SecondaryAttribute.AttrID), strategy.SecondaryAttribute.AttrID)
	} else {
		logger.GetGlobalLogger("shein/product").Infof("✅ 无需策略适配")
	}

	// 验证属性策略的有效性
	logger.GetGlobalLogger("shein/product").Infof("🔍 验证属性策略有效性...")
	validator := NewSKCValidationUtils(b.taskContext)
	if err := validator.ValidateAttributeStrategy(strategy, *ctx.SaleSpecResult); err != nil {
		logger.GetGlobalLogger("shein/product").Warnf("⚠️ 属性策略验证警告: %v", err)
	} else {
		logger.GetGlobalLogger("shein/product").Infof("✅ 属性策略验证通过")
	}

	// 特殊处理单变体情况
	if len(ctx.SaleSpecResult.Variants) == 1 {
		logger.GetGlobalLogger("shein/product").Infof("🎯 检测到单变体情况，使用单变体构建流程")
		processor := NewSKCVariantProcessor(b.imageProcessor, b.attributeMapper, b.skuBuilder, b.taskContext, b.openaiClient)
		return processor.BuildSingleVariantSKC(ctx, strategy)
	}

	// 构建多变体SKC列表
	logger.GetGlobalLogger("shein/product").Infof("🎯 检测到多变体情况，使用多变体构建流程")
	processor := NewSKCVariantProcessor(b.imageProcessor, b.attributeMapper, b.skuBuilder, b.taskContext, b.openaiClient)
	return processor.BuildMultiVariantSKCList(ctx, strategy, b.variantMatcher)
}

// adaptStrategy 根据分类限制适配销售属性
func (b *SKCBuilder) adaptStrategy(
	strategy sheinattr.AttributeStrategy,
	categoryID int,
) (sheinattr.AttributeStrategy, bool, []string) {
	// 检查是否有限制
	restriction, hasRestriction := b.getCategoryRestrictions()[categoryID]
	if !hasRestriction {
		return strategy, false, nil
	}

	var adaptationReasons []string
	isAdapted := false
	adaptedStrategy := strategy

	// 检查主规格是否被禁止
	if strategy.PrimaryAttribute.AttrID == restriction.ForbiddenPrimarySpec {
		logger.GetGlobalLogger("shein/product").Warnf("主规格属性 %d 在分类 %d 中被禁止，需要适配",
			strategy.PrimaryAttribute.AttrID, categoryID)

		// 创建默认颜色主规格
		defaultColorAttr := sheinattr.ResultAttribute{
			AttrID: restriction.DefaultPrimarySpec,
			AttrValue: []sheinattr.AttributeValue{
				{
					ID:    -1,
					Value: "Multi-Color",
				},
			},
		}

		// 将原主规格降级为次规格
		adaptedStrategy.SecondaryAttribute = strategy.PrimaryAttribute
		adaptedStrategy.PrimaryAttribute = defaultColorAttr

		adaptationReasons = append(adaptationReasons,
			fmt.Sprintf("主规格从属性%d替换为属性%d，因为平台限制",
				strategy.PrimaryAttribute.AttrID, restriction.DefaultPrimarySpec))

		isAdapted = true
	}

	// 检查主规格和次规格是否相同
	if adaptedStrategy.PrimaryAttribute.AttrID > 0 &&
		adaptedStrategy.SecondaryAttribute.AttrID > 0 &&
		adaptedStrategy.PrimaryAttribute.AttrID == adaptedStrategy.SecondaryAttribute.AttrID {

		logger.GetGlobalLogger("shein/product").Warnf("检测到规格冲突：主规格和次规格相同 (%d)", adaptedStrategy.PrimaryAttribute.AttrID)

		// 清空次规格以避免冲突
		adaptedStrategy.SecondaryAttribute = sheinattr.ResultAttribute{AttrID: -1}

		adaptationReasons = append(adaptationReasons,
			"清空次规格以避免与主规格冲突")

		isAdapted = true
	}

	if isAdapted {
		logger.GetGlobalLogger("shein/product").Infof("规格适配完成 - 分类: %d, 原主规格: %d, 新主规格: %d",
			categoryID, strategy.PrimaryAttribute.AttrID, adaptedStrategy.PrimaryAttribute.AttrID)
	}

	return adaptedStrategy, isAdapted, adaptationReasons
}

// getCategoryRestrictions 获取分类限制
func (b *SKCBuilder) getCategoryRestrictions() map[int]category.CategoryRestriction {
	// 如果没有设置taskContext，返回空的限制映射
	if b.taskContext == nil || b.taskContext.ManagementClientMgr == nil {
		logger.GetGlobalLogger("shein/product").Warn("TaskContext或ManagementClientMgr为空，返回空的分类限制映射")
		return make(map[int]category.CategoryRestriction)
	}

	// 创建分类限制映射
	categoryRestrictions := make(map[int]category.CategoryRestriction)

	// 获取所有已确认的品类限制集合
	restrictions, err := b.taskContext.ManagementClientMgr.GetCategoryRestrictionCache().GetConfirmedListByPlatform("Shein")
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("获取品类限制集合失败: %v", err)
		// 返回空的限制映射而不是nil
		return categoryRestrictions
	}

	// 将获取到的限制转换为内部格式
	for _, restriction := range restrictions {
		categoryRestrictions[restriction.CategoryId] = category.CategoryRestriction{
			CategoryID:           restriction.CategoryId,
			ForbiddenPrimarySpec: restriction.ForbiddenAttributeId,
			DefaultPrimarySpec:   restriction.DefaultAttributeId,
			PlatformName:         restriction.PlatformName,
		}
	}

	logger.GetGlobalLogger("shein/product").Infof("成功获取%d个分类限制", len(categoryRestrictions))
	return categoryRestrictions
}

// getAttributeNameSafe 安全获取属性名称
func (b *SKCBuilder) getAttributeNameSafe(attrID int) string {
	switch attrID {
	case 27:
		return "Color"
	case 87:
		return "Size"
	case 1001365:
		return "Scent Type"
	case 1001410:
		return "Material"
	case 1001366:
		return "Style"
	default:
		return fmt.Sprintf("Attribute_%d", attrID)
	}
}
