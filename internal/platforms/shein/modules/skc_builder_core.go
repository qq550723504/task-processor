// Package modules 提供SHEIN平台SKC构建核心功能
package modules

import (
	"fmt"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SKCBuilder SKC构建器
type SKCBuilder struct {
	imageProcessor  *ImageProcessor
	attributeMapper *AttributeMapper
	variantMatcher  *VariantMatcher
	skuBuilder      *SKUBuilder
	taskContext     *TaskContext // 添加TaskContext字段
	openaiConfig    *openaiClient.ClientConfig
}

// NewSKCBuilder 创建新的SKC构建器
func NewSKCBuilder(imageProcessor *ImageProcessor, attributeMapper *AttributeMapper, variantMatcher *VariantMatcher, skuBuilder *SKUBuilder, openaiConfig *openaiClient.ClientConfig) *SKCBuilder {
	return &SKCBuilder{
		imageProcessor:  imageProcessor,
		attributeMapper: attributeMapper,
		variantMatcher:  variantMatcher,
		skuBuilder:      skuBuilder,
		openaiConfig:    openaiConfig,
	}
}

// BuildSKCListWithSpecAdaptation 构建SKC列表并进行规格适配
func (b *SKCBuilder) BuildSKCListWithSpecAdaptation(ctx *TaskContext, strategyHandler *AttributeStrategyHandler) ([]product.SKC, []attribute.CustomAttributeRelation, error) {
	logrus.Infof("🏗️ === 开始SKC构建流程 ===")

	// 设置任务上下文
	b.taskContext = ctx
	logrus.Infof("✅ 任务上下文设置完成")

	logrus.Infof("📊 获取动态属性优先级配置...")
	config := strategyHandler.GetDynamicAttributePriorityConfig(ctx.AttributeTemplates)
	logrus.Infof("✅ 动态属性优先级配置获取完成")

	logrus.Infof("🎯 确定属性策略...")
	baseStrategy := strategyHandler.DetermineAttributeStrategy(*ctx.SaleSpecResult, config, ctx.AttributeTemplates)

	// 适配策略
	logrus.Infof("🔄 开始策略适配...")
	strategy, isAdapted, adaptationReasons := b.adaptStrategy(baseStrategy, ctx.ProductData.CategoryID)

	if isAdapted {
		logrus.Infof("✅ 策略适配完成")
		for i, reason := range adaptationReasons {
			logrus.Infof("  适配原因%d: %s", i+1, reason)
		}
		logrus.Infof("适配后策略: 主规格=%s(%d), 次规格=%s(%d)",
			b.getAttributeNameSafe(strategy.PrimaryAttribute.AttrID), strategy.PrimaryAttribute.AttrID,
			b.getAttributeNameSafe(strategy.SecondaryAttribute.AttrID), strategy.SecondaryAttribute.AttrID)
	} else {
		logrus.Infof("✅ 无需策略适配")
	}

	// 验证属性策略的有效性
	logrus.Infof("🔍 验证属性策略有效性...")
	validator := NewSKCValidationUtils(b.taskContext)
	if err := validator.ValidateAttributeStrategy(strategy, *ctx.SaleSpecResult); err != nil {
		logrus.Warnf("⚠️ 属性策略验证警告: %v", err)
	} else {
		logrus.Infof("✅ 属性策略验证通过")
	}

	// 特殊处理单变体情况
	if len(ctx.SaleSpecResult.Variants) == 1 {
		logrus.Infof("🎯 检测到单变体情况，使用单变体构建流程")
		processor := NewSKCVariantProcessor(b.imageProcessor, b.attributeMapper, b.skuBuilder, b.taskContext, b.openaiConfig)
		return processor.BuildSingleVariantSKC(ctx, strategy)
	}

	// 构建多变体SKC列表
	logrus.Infof("🎯 检测到多变体情况，使用多变体构建流程")
	processor := NewSKCVariantProcessor(b.imageProcessor, b.attributeMapper, b.skuBuilder, b.taskContext, b.openaiConfig)
	return processor.BuildMultiVariantSKCList(ctx, strategy, b.variantMatcher)
}

// adaptStrategy 根据分类限制适配销售属性
func (b *SKCBuilder) adaptStrategy(
	strategy AttributeStrategy,
	categoryID int,
) (AttributeStrategy, bool, []string) {
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
		logrus.Warnf("主规格属性 %d 在分类 %d 中被禁止，需要适配",
			strategy.PrimaryAttribute.AttrID, categoryID)

		// 创建默认颜色主规格
		defaultColorAttr := ResultAttribute{
			AttrID: restriction.DefaultPrimarySpec,
			AttrValue: []AttributeValue{
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

		logrus.Warnf("检测到规格冲突：主规格和次规格相同 (%d)", adaptedStrategy.PrimaryAttribute.AttrID)

		// 清空次规格以避免冲突
		adaptedStrategy.SecondaryAttribute = ResultAttribute{AttrID: -1}

		adaptationReasons = append(adaptationReasons,
			"清空次规格以避免与主规格冲突")

		isAdapted = true
	}

	if isAdapted {
		logrus.Infof("规格适配完成 - 分类: %d, 原主规格: %d, 新主规格: %d",
			categoryID, strategy.PrimaryAttribute.AttrID, adaptedStrategy.PrimaryAttribute.AttrID)
	}

	return adaptedStrategy, isAdapted, adaptationReasons
}

// getCategoryRestrictions 获取分类限制
func (b *SKCBuilder) getCategoryRestrictions() map[int]CategoryRestriction {
	// 如果没有设置taskContext，返回空的限制映射
	if b.taskContext == nil || b.taskContext.ManagementClientMgr == nil {
		logrus.Warn("TaskContext或ManagementClientMgr为空，返回空的分类限制映射")
		return make(map[int]CategoryRestriction)
	}

	// 创建分类限制映射
	categoryRestrictions := make(map[int]CategoryRestriction)

	// 获取所有已确认的品类限制集合
	restrictions, err := b.taskContext.ManagementClientMgr.GetCategoryRestrictionCache().GetConfirmedListByPlatform("Shein")
	if err != nil {
		logrus.Errorf("获取品类限制集合失败: %v", err)
		// 返回空的限制映射而不是nil
		return categoryRestrictions
	}

	// 将获取到的限制转换为内部格式
	for _, restriction := range restrictions {
		categoryRestrictions[restriction.CategoryId] = CategoryRestriction{
			CategoryID:           restriction.CategoryId,
			ForbiddenPrimarySpec: restriction.ForbiddenAttributeId,
			DefaultPrimarySpec:   restriction.DefaultAttributeId,
			PlatformName:         restriction.PlatformName,
		}
	}

	logrus.Infof("成功获取%d个分类限制", len(categoryRestrictions))
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
