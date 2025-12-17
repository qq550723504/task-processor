package modules

import (
	"fmt"
	"strings"
	"task-processor/internal/common/shein/api/attribute"
	"task-processor/internal/common/shein/api/product"

	"github.com/sirupsen/logrus"
)

// SKCBuilder SKC构建器
type SKCBuilder struct {
	imageProcessor  *ImageProcessor
	attributeMapper *AttributeMapper
	variantMatcher  *VariantMatcher
	skuBuilder      *SKUBuilder
	taskContext     *TaskContext // 添加TaskContext字段
}

// NewSKCBuilder 创建新的SKC构建器
func NewSKCBuilder(imageProcessor *ImageProcessor, attributeMapper *AttributeMapper, variantMatcher *VariantMatcher, skuBuilder *SKUBuilder) *SKCBuilder {
	return &SKCBuilder{
		imageProcessor:  imageProcessor,
		attributeMapper: attributeMapper,
		variantMatcher:  variantMatcher,
		skuBuilder:      skuBuilder,
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
	if err := b.validateAttributeStrategy(strategy, *ctx.SaleSpecResult); err != nil {
		logrus.Warnf("⚠️ 属性策略验证警告: %v", err)
	} else {
		logrus.Infof("✅ 属性策略验证通过")
	}

	// 特殊处理单变体情况
	if len(ctx.SaleSpecResult.Variants) == 1 {
		logrus.Infof("🎯 检测到单变体情况，使用单变体构建流程")
		return b.buildSingleVariantSKC(ctx, strategy)
	}

	// 构建多变体SKC列表
	logrus.Infof("🎯 检测到多变体情况，使用多变体构建流程")
	return b.buildMultiVariantSKCList(ctx, strategy)
}

// buildSingleVariantSKC 构建单变体SKC
func (b *SKCBuilder) buildSingleVariantSKC(ctx *TaskContext, strategy AttributeStrategy) ([]product.SKC, []attribute.CustomAttributeRelation, error) {
	logrus.Infof("🎯 === 开始单变体SKC构建流程 ===")

	variant := ctx.SaleSpecResult.Variants[0]
	logrus.Infof("📊 单变体信息: ASIN=%s, 策略类型=%s, 价格=%.2f", variant.ASIN, strategy.StrategyType, variant.Price)

	var customAttributeRelations []attribute.CustomAttributeRelation

	// 1. 预处理属性值ID映射
	logrus.Infof("🔄 步骤1: 开始属性值ID映射...")
	mappingRelations, err := b.attributeMapper.MapAttributeValuesToSheinIDs(ctx, &strategy)
	if err != nil {
		logrus.Errorf("❌ 单变体模式 - 属性值ID映射失败: %v", err)
		return nil, nil, fmt.Errorf("属性值ID映射失败: %w", err)
	}
	customAttributeRelations = append(customAttributeRelations, mappingRelations...)
	logrus.Infof("✅ 属性值ID映射完成，创建了 %d 个关系", len(mappingRelations))

	// 检查主要属性值是否有效
	logrus.Infof("🔍 步骤2: 检查主要属性值有效性...")
	if len(strategy.PrimaryAttribute.AttrValue) > 0 {
		primaryAttrValue := strategy.PrimaryAttribute.AttrValue[0]
		logrus.Infof("📋 主要属性值: %s (ID: %d)", primaryAttrValue.Value, primaryAttrValue.ID.Int())
		if primaryAttrValue.ID.Int() <= 0 {
			logrus.Errorf("❌ 单变体模式 - 主要属性值ID无效: %s (ID: %d)", primaryAttrValue.Value, primaryAttrValue.ID.Int())
			return nil, nil, fmt.Errorf("主要属性值ID无效: %s", primaryAttrValue.Value)
		}
		logrus.Infof("✅ 主要属性值ID有效")
	} else {
		logrus.Errorf("❌ 没有主要属性值")
		return nil, nil, fmt.Errorf("没有主要属性值")
	}

	// 3. 构建单变体SKC
	logrus.Infof("🏗️ 步骤3: 开始构建单变体SKC...")
	skcList, builderRelations, err := b.buildSingleVariantDirect(ctx, variant, strategy)
	if err != nil {
		logrus.Errorf("❌ 单变体直接SKC构建失败: %v", err)
		return nil, nil, err
	}
	logrus.Infof("✅ 单变体直接SKC构建成功 - 数量: %d", len(skcList))

	// 合并自定义属性关系
	allRelations := append(customAttributeRelations, builderRelations...)
	logrus.Infof("🎉 单变体SKC构建完成 - SKC数量: %d, 关系数量: %d", len(skcList), len(allRelations))

	return skcList, allRelations, nil
}

// buildMultiVariantSKCList 构建多变体SKC列表
func (b *SKCBuilder) buildMultiVariantSKCList(ctx *TaskContext, strategy AttributeStrategy) ([]product.SKC, []attribute.CustomAttributeRelation, error) {
	logrus.Infof("🔨 === 开始多变体SKC构建流程 ===")

	// 预分配容量
	skcList := make([]product.SKC, 0, len(strategy.PrimaryAttribute.AttrValue))
	var customAttributeRelations []attribute.CustomAttributeRelation

	// 使用 map 来跟踪已处理的主要属性值（按名称去重）
	processedValues := make(map[string]bool)
	// 使用 map 来跟踪已使用的主要属性值ID（按ID去重，避免SHEIN主销售属性重复错误）
	usedAttributeValueIDs := make(map[int]bool)

	// 1. 预处理属性值ID映射 - 将Amazon属性值映射到SHEIN平台属性值ID
	logrus.Infof("🔄 步骤1: 开始预处理属性值ID映射...")
	mappingRelations, err := b.attributeMapper.MapAttributeValuesToSheinIDs(ctx, &strategy)
	if err != nil {
		logrus.Errorf("❌ 属性值ID映射失败: %v", err)
		return nil, nil, fmt.Errorf("属性值ID映射失败: %w", err)
	}
	customAttributeRelations = append(customAttributeRelations, mappingRelations...)
	logrus.Infof("✅ 属性值ID映射完成，创建了 %d 个自定义属性关系", len(mappingRelations))

	// 1.5 修复变体属性：确保所有变体都包含主规格和次规格属性
	b.ensureVariantsHaveRequiredAttributes(ctx, &strategy)

	// 构建SKC列表，遍历主要属性来区分构建多个SKC
	skcsCreated := 0
	logrus.Infof("🔄 步骤2: 开始构建SKC列表，主要属性值数量: %d", len(strategy.PrimaryAttribute.AttrValue))

	for i := 0; i < len(strategy.PrimaryAttribute.AttrValue); i++ {
		attribute := &strategy.PrimaryAttribute.AttrValue[i] // 获取引用而不是副本

		logrus.Infof("🔍 处理主要属性值[%d/%d]: %s (ID: %d)", i+1, len(strategy.PrimaryAttribute.AttrValue), attribute.Value, attribute.ID.Int())

		// 第一层去重检查：按属性值名称去重
		if processedValues[attribute.Value] {
			logrus.Debugf("⏭️ 跳过重复属性值名称: %s", attribute.Value)
			continue
		}

		// 第二层去重检查：按属性值ID去重（关键修复：避免SHEIN主销售属性重复错误）
		attributeValueID := attribute.ID.Int()
		if usedAttributeValueIDs[attributeValueID] {
			logrus.Warnf("⏭️ 跳过重复的主要属性值ID: %d (属性值: %s)，避免SHEIN主销售属性重复错误", attributeValueID, attribute.Value)
			continue
		}

		processedValues[attribute.Value] = true
		usedAttributeValueIDs[attributeValueID] = true

		// 检查属性值ID是否有效
		if attribute.ID.Int() <= 0 {
			logrus.Warnf("⚠️ 跳过无效的属性值: %s (ID: %d)", attribute.Value, attribute.ID.Int())
			continue
		}

		// 查找匹配的变体
		logrus.Debugf("🔍 查找匹配的变体，属性ID: %d, 属性值: %s", strategy.PrimaryAttribute.AttrID, attribute.Value)
		matchedVariants := b.variantMatcher.FindMatchingVariants(ctx,
			ctx.SaleSpecResult.Variants,
			strategy.PrimaryAttribute.AttrID,
			attribute.Value,
		)
		logrus.Infof("📊 属性值 %s 匹配到的变体数量: %d", attribute.Value, len(matchedVariants))

		if len(matchedVariants) == 0 {
			logrus.Warnf("❌ 找不到主要属性值 %s 对应的变体", attribute.Value)
			continue
		}

		// 构建图片信息
		logrus.Debugf("🖼️ 构建图片信息...")
		imagesToUse, err := b.getVariantSpecificImages(ctx, matchedVariants[0])
		if err != nil {
			logrus.Warnf("⚠️ 获取变体特定图片失败，使用产品图片: %v", err)
			imagesToUse = ctx.AmazonProduct.Images
		}

		imageInfo, err := b.imageProcessor.BuildImageInfo(ctx, imagesToUse)
		if err != nil {
			logrus.Errorf("❌ 构建图片信息失败: %v, ASIN: %s", err, ctx.AmazonProduct.Asin)
			// 图片构建失败时使用空的图片信息，不影响SKC创建流程
			imageInfo = product.ImageInfo{}
		} else {
			logrus.Debugf("✅ 成功构建图片信息，图片数量: %d", len(imageInfo.ImageInfoList))
		}

		// 构建SKU列表
		logrus.Debugf("🔧 构建SKU列表...")
		skuBuildReq := SKUBuildRequest{
			SaleAttributeData: *ctx.SaleSpecResult,
			Strategy:          strategy,
			PrimaryAttrValue:  attribute.Value,
			WarehouseCode:     ctx.Warehouses.Data[0].WarehouseCode,
		}
		skuList, err := b.skuBuilder.BuildSKUListWithStrategy(ctx, skuBuildReq)
		if err != nil {
			logrus.Errorf("❌ 构建SKU列表失败: %v", err)
			return nil, nil, fmt.Errorf("failed to build SKU list: %w", err)
		}

		// 如果SKU列表为空，跳过该主要属性值的SKC创建
		if len(skuList) == 0 {
			logrus.Warnf("⚠️ 主要属性值 %s 没有有效的SKU，跳过该SKC - 变体数量: %d", attribute.Value, len(matchedVariants))
			continue
		}

		// 使用统一的SKC创建函数
		logrus.Debugf("🏗️ 创建SKC...")
		skc := b.createSKC(ctx, SKCCreationParams{
			AttributeID:      strategy.PrimaryAttribute.AttrID,
			AttributeValueID: attribute.ID.Int(),
			SKUS:             skuList,
			ImageInfo:        imageInfo,
			SupplierCode:     "", //todo:
			Sort:             i + 1,
		})

		skcList = append(skcList, skc)
		logrus.Infof("✅ 成功创建SKC[%d]: 属性值=%s, SKU数量=%d", skcsCreated+1, attribute.Value, len(skuList))
		skcsCreated++
	}

	logrus.Infof("🎉 多变体SKC构建完成 - 成功创建: %d 个SKC", len(skcList))
	return skcList, customAttributeRelations, nil
}

// buildSingleVariantDirect 构建单变体直接SKC
func (b *SKCBuilder) buildSingleVariantDirect(ctx *TaskContext, variant Variant, strategy AttributeStrategy) ([]product.SKC, []attribute.CustomAttributeRelation, error) {
	var customAttributeRelations []attribute.CustomAttributeRelation

	imageInfo, err := b.imageProcessor.BuildImageInfo(ctx, ctx.AmazonProduct.Images)
	if err != nil {
		return nil, nil, fmt.Errorf("构建图片信息失败: %w", err)
	}

	// 使用第一个可用的主要属性值
	if len(strategy.PrimaryAttribute.AttrValue) == 0 {
		return nil, nil, fmt.Errorf("主要属性没有可用的属性值")
	}

	primaryAttrValue := strategy.PrimaryAttribute.AttrValue[0]

	// 检查主要属性值ID是否有效（应该在预处理阶段已经映射完成）
	if primaryAttrValue.ID.Int() <= 0 {
		return nil, nil, fmt.Errorf("主要属性值ID无效: %s (ID: %d)，应该在预处理阶段已经映射", primaryAttrValue.Value, primaryAttrValue.ID.Int())
	}

	// 构建SKU列表 - 直接使用变体信息，不需要匹配
	skuList, err := b.skuBuilder.BuildSKUListForSingleVariant(ctx, variant, strategy)
	if err != nil {
		return nil, nil, fmt.Errorf("构建SKU列表失败: %w", err)
	}

	// 如果SKU列表为空，返回错误
	if len(skuList) == 0 {
		return nil, nil, fmt.Errorf("无法为单变体创建有效的SKU")
	}

	// SHEIN规则验证：只有主规格没有次规格时，一个SKC下只能有一个SKU
	hasSecondaryAttribute := strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0
	if !hasSecondaryAttribute && len(skuList) > 1 {
		logrus.Warnf("单变体直接构建违反SHEIN规则：只有主规格没有次规格，但创建了%d个SKU，只保留第一个", len(skuList))
		skuList = skuList[:1] // 只保留第一个SKU
	}

	// 使用统一的SKC创建函数
	skc := b.createSKC(ctx, SKCCreationParams{
		AttributeID:      strategy.PrimaryAttribute.AttrID,
		AttributeValueID: primaryAttrValue.ID.Int(),
		SKUS:             skuList,
		ImageInfo:        imageInfo,
		SupplierCode:     "", // 此场景下不需要SupplierCode
		Sort:             1,
	})

	logrus.Infof("成功直接构建单变体SKC，包含%d个SKU (主规格: %d, 次规格: %v)",
		len(skuList), strategy.PrimaryAttribute.AttrID, hasSecondaryAttribute)
	return []product.SKC{skc}, customAttributeRelations, nil
}

// createSKC 创建SKC的工厂函数
func (b *SKCBuilder) createSKC(ctx *TaskContext, params SKCCreationParams) product.SKC {
	// 1. 获取目标语言列表
	targetLanguages := GetTargetLanguagesByRegion(ctx.Task.Region)

	// 2. 查找标题作为翻译源
	sourceTitle := b.findBestSourceTitle(ctx, params)

	// 3. 检测源标题的语言
	sourceLang := b.detectTitleLanguage(sourceTitle)

	// 4. 初始化多语言内容结构
	multiLanguageNameList := b.initializeMultiLanguageContent(targetLanguages)

	// 5. 翻译到所有目标语言
	b.translateToAllLanguages(ctx, sourceTitle, sourceLang, &multiLanguageNameList)

	// 6. 选择主要显示语言
	primaryLanguageContent := b.selectPrimaryDisplayLanguage(targetLanguages, multiLanguageNameList, sourceTitle)

	skc := product.SKC{
		SaleAttribute: product.SaleAttribute{
			AttributeID:        params.AttributeID,
			AttributeValueID:   params.AttributeValueID,
			IsSPPSaleAttribute: false,
			PreFillSpec:        false,
		},
		SKUS:                    params.SKUS,
		ImageInfo:               params.ImageInfo,
		SiteDetailImageInfoList: []product.SiteDetailImageInfo{},
		ShelfWay:                1,
		ShelfRequire:            0,
		MultiLanguageName:       primaryLanguageContent,
		MultiLanguageNameList:   multiLanguageNameList,
		Sort:                    params.Sort,
	}
	return skc
}

// findBestSourceTitle 查找最佳的源标题作为翻译源
func (b *SKCBuilder) findBestSourceTitle(ctx *TaskContext, params SKCCreationParams) string {
	logrus.Debugf("🔍 开始查找源标题...")

	// 从变体中查找标题
	if ctx.Variants != nil && len(*ctx.Variants) > 0 && len(params.SKUS) > 0 {
		for _, variant := range *ctx.Variants {
			if variant.Title != "" {
				logrus.Infof("✅ 找到变体标题: %s", variant.Title)
				return variant.Title
			}
		}
	}

	// 如果没有找到变体标题，尝试使用产品标题
	if ctx.AmazonProduct.Title != "" {
		logrus.Infof("✅ 使用产品标题: %s", ctx.AmazonProduct.Title)
		return ctx.AmazonProduct.Title
	}

	logrus.Warnf("⚠️ 未找到有效的标题")
	return ""
}

// detectTitleLanguage 检测标题的语言
func (b *SKCBuilder) detectTitleLanguage(title string) string {
	title = strings.TrimSpace(title)

	if title == "" {
		return "en" // 默认返回英文
	}

	// 简单的语言检测：统计不同字符集的字符数量
	var japaneseCount, chineseCount, englishCount int

	for _, r := range title {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF): // 平假名和片假名
			japaneseCount++
		case r >= 0x4E00 && r <= 0x9FFF: // 中日韩统一表意文字
			chineseCount++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			englishCount++
		}
	}

	// 判断主要语言
	if japaneseCount > chineseCount && japaneseCount > englishCount {
		logrus.Infof("🔍 检测到标题语言: 日语")
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		logrus.Infof("🔍 检测到标题语言: 中文")
		return "zh"
	}

	logrus.Infof("🔍 检测到标题语言: 英文")
	return "en"
}

// initializeMultiLanguageContent 初始化多语言内容结构
func (b *SKCBuilder) initializeMultiLanguageContent(targetLanguages []string) []product.LanguageContent {
	logrus.Debugf("🌐 初始化多语言内容结构，目标语言数量: %d", len(targetLanguages))

	multiLanguageNameList := make([]product.LanguageContent, 0, len(targetLanguages))

	for _, lang := range targetLanguages {
		multiLanguageNameList = append(multiLanguageNameList, product.LanguageContent{
			Language: lang,
			Name:     "", // 初始化为空，后续通过翻译填充
		})
		logrus.Debugf("📝 初始化语言: %s", lang)
	}

	return multiLanguageNameList
}

// translateToAllLanguages 翻译到所有目标语言
func (b *SKCBuilder) translateToAllLanguages(ctx *TaskContext, sourceTitle string, sourceLang string, multiLanguageNameList *[]product.LanguageContent) {
	if ctx.ShopClient == nil || sourceTitle == "" {
		logrus.Warnf("⚠️ 跳过翻译：ShopClient为空(%v) 或 源标题为空(%v)", ctx.ShopClient == nil, sourceTitle == "")
		return
	}

	for i := range *multiLanguageNameList {
		langContent := &(*multiLanguageNameList)[i]

		// 如果目标语言与源语言相同，直接设置原标题
		if langContent.Language == sourceLang {
			langContent.Name = sourceTitle
			logrus.Debugf("✅ 设置源语言(%s)标题: %s", sourceLang, sourceTitle)
			continue
		}

		// 翻译到目标语言
		translatedTitle, err := ctx.ShopClient.Translate(sourceTitle, sourceLang, langContent.Language)
		if err != nil {
			logrus.Warnf("❌ 翻译到语言 %s 失败: %v，使用源标题作为后备", langContent.Language, err)
			langContent.Name = sourceTitle // 翻译失败时使用源标题作为后备
			continue
		}

		langContent.Name = translatedTitle
	}
}

// selectPrimaryDisplayLanguage 选择主要显示语言
func (b *SKCBuilder) selectPrimaryDisplayLanguage(targetLanguages []string, multiLanguageNameList []product.LanguageContent, sourceTitle string) product.LanguageContent {
	if len(targetLanguages) == 0 {
		// 如果没有目标语言，尝试从多语言列表中选择第一个有效的
		if len(multiLanguageNameList) > 0 && multiLanguageNameList[0].Name != "" {
			logrus.Infof("📋 无目标语言，使用多语言列表第一项作为主要显示语言: %s", multiLanguageNameList[0].Language)
			return multiLanguageNameList[0]
		}
		// 最后的后备方案
		logrus.Infof("📋 无目标语言，使用源标题作为主要显示语言")
		return product.LanguageContent{
			Language: "en",
			Name:     sourceTitle,
		}
	}

	// 使用第一个目标语言作为主要显示语言
	primaryTargetLang := targetLanguages[0]
	logrus.Infof("🎯 选择主要显示语言: %s", primaryTargetLang)

	// 在多语言列表中查找对应的翻译内容
	for _, langContent := range multiLanguageNameList {
		if langContent.Language == primaryTargetLang && langContent.Name != "" {
			logrus.Infof("✅ 使用目标语言 %s 作为主要显示标题: %s", primaryTargetLang, langContent.Name)
			return langContent
		}
	}

	// 如果没有找到目标语言的翻译，使用源标题作为后备
	logrus.Warnf("⚠️ 未找到语言 %s 的翻译内容，使用源标题作为后备", primaryTargetLang)
	return product.LanguageContent{
		Language: primaryTargetLang,
		Name:     sourceTitle,
	}
}

// getVariantSpecificImages 从变体数据中获取变体特定的图片
func (b *SKCBuilder) getVariantSpecificImages(ctx *TaskContext, variant Variant) ([]string, error) {
	// 如果变体ASIN与主产品相同，使用主产品图片
	if variant.ASIN == ctx.AmazonProduct.Asin {
		logrus.Infof("变体ASIN与主产品相同，使用主产品图片，ASIN: %s", variant.ASIN)
		return ctx.AmazonProduct.Images, nil
	}

	// 尝试从fallback产品的变体数据中查找图片
	for _, variation := range *ctx.Variants {
		if variation.Asin == variant.ASIN {
			if len(variation.Images) >= 3 {
				return variation.Images, nil
			} else if len(variation.Images) > 0 {
				// 如果变体图片少于3张，补充主产品图片
				combinedImages := make([]string, len(variation.Images))
				copy(combinedImages, variation.Images)
				for _, img := range ctx.AmazonProduct.Images {
					// 避免重复添加相同的图片
					found := false
					for _, existingImg := range combinedImages {
						if existingImg == img {
							found = true
							break
						}
					}
					if !found {
						combinedImages = append(combinedImages, img)
					}
				}
				return combinedImages, nil
			}
			// 找到匹配的变体但没有图片，跳出循环继续查找其他变体
			break
		}
	}

	// 如果没有找到特定变体的图片，返回主产品图片
	logrus.Infof("未找到变体 %s 的特定图片，使用主产品图片", variant.ASIN)
	return ctx.AmazonProduct.Images, nil
}

// Helper methods

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

// validateAttributeStrategy 验证属性策略的有效性
func (b *SKCBuilder) validateAttributeStrategy(strategy AttributeStrategy, saleAttributeData ResultSaleAttribute) error {
	var warnings []string

	// 验证主要属性
	if strategy.PrimaryAttribute.AttrID <= 0 {
		warnings = append(warnings, "主要属性ID无效")
	} else if len(strategy.PrimaryAttribute.AttrValue) == 0 {
		warnings = append(warnings, "主要属性值为空")
	}

	// 验证次要属性（如果存在）
	hasSecondaryAttribute := strategy.SecondaryAttribute.AttrID > 0 && len(strategy.SecondaryAttribute.AttrValue) > 0
	if hasSecondaryAttribute {
		// 检查次要属性值是否在变体中存在
		secondaryAttrNames := []string{"size", "Size", "尺寸", "尺码"}
		if strategy.SecondaryAttribute.AttrID == 27 {
			secondaryAttrNames = []string{"color", "Color", "颜色"}
		}

		matchedCount := 0
		totalValues := len(strategy.SecondaryAttribute.AttrValue)

		for _, attrValue := range strategy.SecondaryAttribute.AttrValue {
			found := false
			for _, variant := range saleAttributeData.Variants {
				for _, attrName := range secondaryAttrNames {
					if variantValue, exists := variant.Attributes[attrName]; exists {
						if strings.EqualFold(variantValue, attrValue.Value) {
							found = true
							break
						}
					}
				}
				if found {
					break
				}
			}
			if found {
				matchedCount++
			}
		}

		validationRate := float64(matchedCount) / float64(totalValues)
		if validationRate < 0.3 {
			warnings = append(warnings, fmt.Sprintf("次要属性值在变体中的匹配率过低: %.1f%% (%d/%d)",
				validationRate*100, matchedCount, totalValues))
		}

		logrus.Infof("次要属性验证结果: 属性ID=%d, 匹配率=%.1f%% (%d/%d)",
			strategy.SecondaryAttribute.AttrID, validationRate*100, matchedCount, totalValues)
	}

	// 验证变体数据完整性
	validVariantCount := 0
	for _, variant := range saleAttributeData.Variants {
		if variant.Price > 0 && variant.ASIN != "" {
			validVariantCount++
		}
	}

	if validVariantCount == 0 {
		warnings = append(warnings, "没有有效的变体数据（价格>0且ASIN不为空）")
	} else if float64(validVariantCount)/float64(len(saleAttributeData.Variants)) < 0.5 {
		warnings = append(warnings, fmt.Sprintf("有效变体比例过低: %.1f%% (%d/%d)",
			float64(validVariantCount)*100/float64(len(saleAttributeData.Variants)),
			validVariantCount, len(saleAttributeData.Variants)))
	}

	if len(warnings) > 0 {
		return fmt.Errorf("策略验证发现问题: %s", strings.Join(warnings, "; "))
	}

	logrus.Infof("属性策略验证通过: 策略=%s, 有效变体=%d/%d",
		strategy.StrategyType, validVariantCount, len(saleAttributeData.Variants))
	return nil
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

// ensureVariantsHaveRequiredAttributes 确保所有变体都包含必需的主规格和次规格属性
func (b *SKCBuilder) ensureVariantsHaveRequiredAttributes(ctx *TaskContext, strategy *AttributeStrategy) {
	logrus.Infof("🔧 === 开始修复变体属性 ===")

	// 获取主规格和次规格的属性名
	primaryAttrName := b.getAttributeNameForVariant(strategy.PrimaryAttribute.AttrID)
	secondaryAttrName := ""
	if strategy.SecondaryAttribute.AttrID > 0 {
		secondaryAttrName = b.getAttributeNameForVariant(strategy.SecondaryAttribute.AttrID)
	}

	logrus.Infof("📋 主规格属性: ID=%d, Name=%s", strategy.PrimaryAttribute.AttrID, primaryAttrName)
	if secondaryAttrName != "" {
		logrus.Infof("📋 次规格属性: ID=%d, Name=%s", strategy.SecondaryAttribute.AttrID, secondaryAttrName)
	}

	// 检查并修复每个变体
	fixedCount := 0
	for i := range ctx.SaleSpecResult.Variants {
		variant := &ctx.SaleSpecResult.Variants[i]
		modified := false

		// 检查主规格属性
		if !b.variantHasAttribute(variant, primaryAttrName) {
			// 变体缺少主规格属性，添加默认值
			if len(strategy.PrimaryAttribute.AttrValue) > 0 {
				defaultValue := strategy.PrimaryAttribute.AttrValue[0].Value
				variant.Attributes[primaryAttrName] = defaultValue
				logrus.Infof("✅ 为变体 ASIN=%s 添加主规格属性: %s = %s", variant.ASIN, primaryAttrName, defaultValue)
				modified = true
			}
		}

		// 检查次规格属性（如果存在）
		if secondaryAttrName != "" && !b.variantHasAttribute(variant, secondaryAttrName) {
			// 变体缺少次规格属性，尝试从原始数据中查找
			// 这种情况通常不应该发生，因为次规格应该是变体的区分属性
			logrus.Warnf("⚠️ 变体 ASIN=%s 缺少次规格属性: %s", variant.ASIN, secondaryAttrName)
		}

		if modified {
			fixedCount++
		}
	}

	if fixedCount > 0 {
		logrus.Infof("🎉 变体属性修复完成，共修复 %d 个变体", fixedCount)
	} else {
		logrus.Infof("✅ 所有变体属性完整，无需修复")
	}
}

// getAttributeNameForVariant 获取属性在变体中使用的名称
func (b *SKCBuilder) getAttributeNameForVariant(attrID int) string {
	// 从属性模板中查找属性名
	if b.taskContext != nil && b.taskContext.AttributeTemplates != nil {
		for _, data := range b.taskContext.AttributeTemplates.Data {
			for _, attrInfo := range data.AttributeInfos {
				if attrInfo.AttributeID == attrID {
					// 优先使用英文名称
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

	// 如果找不到，使用默认映射
	switch attrID {
	case 27:
		return "Color"
	case 87:
		return "Size"
	default:
		return fmt.Sprintf("attr_%d", attrID)
	}
}

// variantHasAttribute 检查变体是否包含指定属性
func (b *SKCBuilder) variantHasAttribute(variant *Variant, attrName string) bool {
	if variant.Attributes == nil {
		return false
	}

	// 检查精确匹配
	if _, exists := variant.Attributes[attrName]; exists {
		return true
	}

	// 检查不区分大小写的匹配
	attrNameLower := strings.ToLower(attrName)
	for key := range variant.Attributes {
		if strings.ToLower(key) == attrNameLower {
			return true
		}
	}

	return false
}
