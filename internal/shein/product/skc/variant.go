// Package skc 提供SHEIN平台SKC变体处理功能
package skc

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein"
	api_attribute "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/api/product"
	"task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/image"
	"task-processor/internal/shein/product/sku"
	"task-processor/internal/shein/product/variant"
)

// SKCVariantProcessor SKC变体处理器
type SKCVariantProcessor struct {
	imageProcessor  *image.ImageProcessor
	attributeMapper *attribute.AttributeMapper
	skuBuilder      *sku.SKUBuilder
	taskContext     *shein.TaskContext
	openaiClient    openaiClient.ChatCompleter
}

// NewSKCVariantProcessor 创建新的SKC变体处理器
func NewSKCVariantProcessor(imageProcessor *image.ImageProcessor, attributeMapper *attribute.AttributeMapper, skuBuilder *sku.SKUBuilder, taskContext *shein.TaskContext, openaiClient openaiClient.ChatCompleter) *SKCVariantProcessor {
	return &SKCVariantProcessor{
		imageProcessor:  imageProcessor,
		attributeMapper: attributeMapper,
		skuBuilder:      skuBuilder,
		taskContext:     taskContext,
		openaiClient:    openaiClient,
	}
}

// BuildSingleVariantSKC 构建单变体SKC
func (p *SKCVariantProcessor) BuildSingleVariantSKC(ctx *shein.TaskContext, strategy shein.AttributeStrategy) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	logger.GetGlobalLogger("shein/product").Infof("🎯 === 开始单变体SKC构建流程 ===")

	variant := ctx.SaleSpecResult.Variants[0]
	logger.GetGlobalLogger("shein/product").Infof("📊 单变体信息: ASIN=%s, 策略类型=%s, 价格=%.2f", variant.ASIN, strategy.StrategyType, variant.Price)

	var customAttributeRelations []api_attribute.CustomAttributeRelation

	// 1. 预处理属性值ID映射
	logger.GetGlobalLogger("shein/product").Infof("🔄 步骤1: 开始属性值ID映射...")
	mappingRelations, err := p.attributeMapper.MapAttributeValuesToSheinIDs(ctx, &strategy)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 单变体模式 - 属性值ID映射失败: %v", err)
		return nil, nil, fmt.Errorf("属性值ID映射失败: %w", err)
	}
	customAttributeRelations = append(customAttributeRelations, mappingRelations...)
	logger.GetGlobalLogger("shein/product").Infof("✅ 属性值ID映射完成，创建了 %d 个关系", len(mappingRelations))

	// 检查主要属性值是否有效
	logger.GetGlobalLogger("shein/product").Infof("🔍 步骤2: 检查主要属性值有效性...")
	if len(strategy.PrimaryAttribute.AttrValue) > 0 {
		primaryAttrValue := strategy.PrimaryAttribute.AttrValue[0]
		logger.GetGlobalLogger("shein/product").Infof("📋 主要属性值: %s (ID: %d)", primaryAttrValue.Value, primaryAttrValue.ID.Int())
		if primaryAttrValue.ID.Int() <= 0 {
			logger.GetGlobalLogger("shein/product").Errorf("❌ 单变体模式 - 主要属性值ID无效: %s (ID: %d)", primaryAttrValue.Value, primaryAttrValue.ID.Int())
			return nil, nil, fmt.Errorf("主要属性值ID无效: %s", primaryAttrValue.Value)
		}
		logger.GetGlobalLogger("shein/product").Infof("✅ 主要属性值ID有效")
	} else {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 没有主要属性值")
		return nil, nil, fmt.Errorf("没有主要属性值")
	}

	// 3. 构建单变体SKC
	logger.GetGlobalLogger("shein/product").Infof("🏗️ 步骤3: 开始构建单变体SKC...")
	skcList, builderRelations, err := p.buildSingleVariantDirect(ctx, variant, strategy)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 单变体直接SKC构建失败: %v", err)
		return nil, nil, err
	}
	logger.GetGlobalLogger("shein/product").Infof("✅ 单变体直接SKC构建成功 - 数量: %d", len(skcList))

	// 合并自定义属性关系
	allRelations := append(customAttributeRelations, builderRelations...)
	logger.GetGlobalLogger("shein/product").Infof("🎉 单变体SKC构建完成 - SKC数量: %d, 关系数量: %d", len(skcList), len(allRelations))

	return skcList, allRelations, nil
}

// BuildMultiVariantSKCList 构建多变体SKC列表
func (p *SKCVariantProcessor) BuildMultiVariantSKCList(ctx *shein.TaskContext, strategy shein.AttributeStrategy, variantMatcher *variant.VariantMatcher) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	logger.GetGlobalLogger("shein/product").Infof("🔨 === 开始多变体SKC构建流程 ===")

	// 预分配容量
	skcList := make([]product.SKC, 0, len(strategy.PrimaryAttribute.AttrValue))
	var customAttributeRelations []api_attribute.CustomAttributeRelation

	// 使用 map 来跟踪已处理的主要属性值（按名称去重）
	processedValues := make(map[string]bool)
	// 使用 map 来跟踪已使用的主要属性值ID（按ID去重，避免SHEIN主销售属性重复错误）
	usedAttributeValueIDs := make(map[int]bool)

	// 1. 预处理属性值ID映射 - 将Amazon属性值映射到SHEIN平台属性值ID
	logger.GetGlobalLogger("shein/product").Infof("🔄 步骤1: 开始预处理属性值ID映射...")
	mappingRelations, err := p.attributeMapper.MapAttributeValuesToSheinIDs(ctx, &strategy)
	if err != nil {
		logger.GetGlobalLogger("shein/product").Errorf("❌ 属性值ID映射失败: %v", err)
		return nil, nil, fmt.Errorf("属性值ID映射失败: %w", err)
	}
	customAttributeRelations = append(customAttributeRelations, mappingRelations...)
	logger.GetGlobalLogger("shein/product").Infof("✅ 属性值ID映射完成，创建了 %d 个自定义属性关系", len(mappingRelations))

	// 1.5 修复变体属性：确保所有变体都包含主规格和次规格属性
	p.ensureVariantsHaveRequiredAttributes(ctx, &strategy)

	// 构建SKC列表，遍历主要属性来区分构建多个SKC
	skcsCreated := 0
	logger.GetGlobalLogger("shein/product").Infof("🔄 步骤2: 开始构建SKC列表，主要属性值数量: %d", len(strategy.PrimaryAttribute.AttrValue))

	for i := 0; i < len(strategy.PrimaryAttribute.AttrValue); i++ {
		attribute := &strategy.PrimaryAttribute.AttrValue[i] // 获取引用而不是副本

		logger.GetGlobalLogger("shein/product").Infof("🔍 处理主要属性值[%d/%d]: %s (ID: %d)", i+1, len(strategy.PrimaryAttribute.AttrValue), attribute.Value, attribute.ID.Int())

		// 第一层去重检查：按属性值名称去重
		if processedValues[attribute.Value] {
			logger.GetGlobalLogger("shein/product").Debugf("⏭️ 跳过重复属性值名称: %s", attribute.Value)
			continue
		}

		// 第二层去重检查：按属性值ID去重（关键修复：避免SHEIN主销售属性重复错误）
		attributeValueID := attribute.ID.Int()
		if usedAttributeValueIDs[attributeValueID] {
			logger.GetGlobalLogger("shein/product").Warnf("⏭️ 跳过重复的主要属性值ID: %d (属性值: %s)，避免SHEIN主销售属性重复错误", attributeValueID, attribute.Value)
			continue
		}

		processedValues[attribute.Value] = true
		usedAttributeValueIDs[attributeValueID] = true

		// 检查属性值ID是否有效
		if attribute.ID.Int() <= 0 {
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 跳过无效的属性值: %s (ID: %d)", attribute.Value, attribute.ID.Int())
			continue
		}

		// 查找匹配的变体
		logger.GetGlobalLogger("shein/product").Debugf("🔍 查找匹配的变体，属性ID: %d, 属性值: %s", strategy.PrimaryAttribute.AttrID, attribute.Value)
		matchedVariants := variantMatcher.FindMatchingVariants(ctx,
			ctx.SaleSpecResult.Variants,
			strategy.PrimaryAttribute.AttrID,
			attribute.Value,
		)
		logger.GetGlobalLogger("shein/product").Infof("📊 属性值 %s 匹配到的变体数量: %d", attribute.Value, len(matchedVariants))

		if len(matchedVariants) == 0 {
			logger.GetGlobalLogger("shein/product").Warnf("❌ 找不到主要属性值 %s 对应的变体", attribute.Value)
			continue
		}

		// 构建图片信息
		logger.GetGlobalLogger("shein/product").Debugf("🖼️ 构建图片信息...")
		imageHandler := NewSKCImageHandler(p.imageProcessor, p.taskContext)
		imagesToUse, err := imageHandler.GetVariantSpecificImages(ctx, matchedVariants[0])
		if err != nil {
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 获取变体特定图片失败，使用产品图片: %v", err)
			imagesToUse = ctx.AmazonProduct.Images
		}

		imageInfo, err := p.imageProcessor.BuildImageInfo(ctx, imagesToUse)
		if err != nil {
			logger.GetGlobalLogger("shein/product").Errorf("❌ 构建图片信息失败: %v, ASIN: %s", err, ctx.AmazonProduct.Asin)
			// 图片构建失败时使用空的图片信息，不影响SKC创建流程
			imageInfo = product.ImageInfo{}
		} else {
			logger.GetGlobalLogger("shein/product").Debugf("✅ 成功构建图片信息，图片数量: %d", len(imageInfo.ImageInfoList))
		}

		// 构建SKU列表
		logger.GetGlobalLogger("shein/product").Debugf("🔧 构建SKU列表...")
		skuBuildReq := shein.SKUBuildRequest{
			SaleAttributeData: *ctx.SaleSpecResult,
			Strategy:          strategy,
			PrimaryAttrValue:  attribute.Value,
			WarehouseCode:     ctx.Warehouses.Data[0].WarehouseCode,
		}
		skuList, err := p.skuBuilder.BuildSKUListWithStrategy(ctx, skuBuildReq)
		if err != nil {
			logger.GetGlobalLogger("shein/product").Errorf("❌ 构建SKU列表失败: %v", err)
			return nil, nil, fmt.Errorf("failed to build SKU list: %w", err)
		}

		// 如果SKU列表为空，跳过该主要属性值的SKC创建
		if len(skuList) == 0 {
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 主要属性值 %s 没有有效的SKU，跳过该SKC - 变体数量: %d", attribute.Value, len(matchedVariants))
			continue
		}

		// 使用统一的SKC创建函数
		logger.GetGlobalLogger("shein/product").Debugf("🏗️ 创建SKC...")
		translationHandler := NewSKCTranslationHandler(p.taskContext, p.openaiClient)
		skc := translationHandler.CreateSKC(ctx, shein.SKCCreationParams{
			AttributeID:      strategy.PrimaryAttribute.AttrID,
			AttributeValueID: attribute.ID.Int(),
			SKUS:             skuList,
			ImageInfo:        imageInfo,
			SupplierCode:     "", //todo:
			Sort:             i + 1,
		})

		// 自动修复多件商品SKU图片
		p.autoFixMultiPieceSKUImages(&skc, &imageInfo)

		skcList = append(skcList, skc)
		logger.GetGlobalLogger("shein/product").Infof("✅ 成功创建SKC[%d]: 属性值=%s, SKU数量=%d", skcsCreated+1, attribute.Value, len(skuList))
		skcsCreated++
	}

	logger.GetGlobalLogger("shein/product").Infof("🎉 多变体SKC构建完成 - 成功创建: %d 个SKC", len(skcList))
	return skcList, customAttributeRelations, nil
}

// buildSingleVariantDirect 构建单变体直接SKC
func (p *SKCVariantProcessor) buildSingleVariantDirect(ctx *shein.TaskContext, variant shein.Variant, strategy shein.AttributeStrategy) ([]product.SKC, []api_attribute.CustomAttributeRelation, error) {
	var customAttributeRelations []api_attribute.CustomAttributeRelation

	imageInfo, err := p.imageProcessor.BuildImageInfo(ctx, ctx.AmazonProduct.Images)
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
	skuList, err := p.skuBuilder.BuildSKUListForSingleVariant(ctx, variant, strategy)
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
		logger.GetGlobalLogger("shein/product").Warnf("单变体直接构建违反SHEIN规则：只有主规格没有次规格，但创建了%d个SKU，只保留第一个", len(skuList))
		skuList = skuList[:1] // 只保留第一个SKU
	}

	// 使用统一的SKC创建函数
	translationHandler := NewSKCTranslationHandler(p.taskContext, p.openaiClient)
	skc := translationHandler.CreateSKC(ctx, shein.SKCCreationParams{
		AttributeID:      strategy.PrimaryAttribute.AttrID,
		AttributeValueID: primaryAttrValue.ID.Int(),
		SKUS:             skuList,
		ImageInfo:        imageInfo,
		SupplierCode:     "", // 此场景下不需要SupplierCode
		Sort:             1,
	})

	// 自动修复多件商品SKU图片
	p.autoFixMultiPieceSKUImages(&skc, &imageInfo)

	logger.GetGlobalLogger("shein/product").Infof("成功直接构建单变体SKC，包含%d个SKU (主规格: %d, 次规格: %v)",
		len(skuList), strategy.PrimaryAttribute.AttrID, hasSecondaryAttribute)
	return []product.SKC{skc}, customAttributeRelations, nil
}

// ensureVariantsHaveRequiredAttributes 确保所有变体都包含必需的主规格和次规格属性
func (p *SKCVariantProcessor) ensureVariantsHaveRequiredAttributes(ctx *shein.TaskContext, strategy *shein.AttributeStrategy) {
	logger.GetGlobalLogger("shein/product").Infof("🔧 === 开始修复变体属性 ===")

	// 获取主规格和次规格的属性名
	primaryAttrName := p.getAttributeNameForVariant(strategy.PrimaryAttribute.AttrID)
	secondaryAttrName := ""
	if strategy.SecondaryAttribute.AttrID > 0 {
		secondaryAttrName = p.getAttributeNameForVariant(strategy.SecondaryAttribute.AttrID)
	}

	logger.GetGlobalLogger("shein/product").Infof("📋 主规格属性: ID=%d, Name=%s", strategy.PrimaryAttribute.AttrID, primaryAttrName)
	if secondaryAttrName != "" {
		logger.GetGlobalLogger("shein/product").Infof("📋 次规格属性: ID=%d, Name=%s", strategy.SecondaryAttribute.AttrID, secondaryAttrName)
	}

	// 检查并修复每个变体
	fixedCount := 0
	for i := range ctx.SaleSpecResult.Variants {
		variant := &ctx.SaleSpecResult.Variants[i]
		modified := false

		// 检查主规格属性
		if !p.variantHasAttribute(variant, primaryAttrName) {
			// 变体缺少主规格属性，添加默认值
			if len(strategy.PrimaryAttribute.AttrValue) > 0 {
				defaultValue := strategy.PrimaryAttribute.AttrValue[0].Value
				variant.Attributes[primaryAttrName] = defaultValue
				logger.GetGlobalLogger("shein/product").Infof("✅ 为变体 ASIN=%s 添加主规格属性: %s = %s", variant.ASIN, primaryAttrName, defaultValue)
				modified = true
			}
		}

		// 检查次规格属性（如果存在）
		if secondaryAttrName != "" && !p.variantHasAttribute(variant, secondaryAttrName) {
			// 变体缺少次规格属性，尝试从原始数据中查找
			// 这种情况通常不应该发生，因为次规格应该是变体的区分属性
			logger.GetGlobalLogger("shein/product").Warnf("⚠️ 变体 ASIN=%s 缺少次规格属性: %s", variant.ASIN, secondaryAttrName)
		}

		if modified {
			fixedCount++
		}
	}

	if fixedCount > 0 {
		logger.GetGlobalLogger("shein/product").Infof("🎉 变体属性修复完成，共修复 %d 个变体", fixedCount)
	} else {
		logger.GetGlobalLogger("shein/product").Infof("✅ 所有变体属性完整，无需修复")
	}
}

// getAttributeNameForVariant 获取属性在变体中使用的名称
func (p *SKCVariantProcessor) getAttributeNameForVariant(attrID int) string {
	// 从属性模板中查找属性名
	if p.taskContext != nil && p.taskContext.AttributeTemplates != nil {
		for _, data := range p.taskContext.AttributeTemplates.Data {
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
func (p *SKCVariantProcessor) variantHasAttribute(variant *shein.Variant, attrName string) bool {
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

// autoFixMultiPieceSKUImages 自动修复多件商品SKU图片
func (p *SKCVariantProcessor) autoFixMultiPieceSKUImages(skc *product.SKC, skcImageInfo *product.ImageInfo) {
	if skc == nil || len(skc.SKUS) == 0 {
		return
	}

	fixer := sku.NewSKUImageAutoFixer()
	fixedCount := 0

	for i := range skc.SKUS {
		sku := &skc.SKUS[i]

		// 检查是否为多件商品
		if !fixer.IsMultiPieceSKU(sku) {
			continue
		}

		// 检查SKU是否已有图片
		if sku.ImageInfo != nil && len(sku.ImageInfo.ImageInfoList) > 0 {
			// 已有图片，只需修复排序
			fixer.AutoFixSKUImageSorting(sku)
			continue
		}

		// 检查SKC是否有图片可以复制
		if skcImageInfo == nil || len(skcImageInfo.ImageInfoList) == 0 {
			logger.GetGlobalLogger("shein/product").Warnf("多件商品SKU %s 缺少图片，但SKC也没有图片可复制", sku.SupplierSKU)
			continue
		}

		// 从SKC复制第一张图片到SKU
		firstImage := skcImageInfo.ImageInfoList[0]
		sku.ImageInfo = &product.ImageInfo{
			ImageGroupCode: nil,
			ImageInfoList: []product.ImageDetail{
				{
					ImageType:             firstImage.ImageType,
					ImageSort:             1, // SKU图片排序固定为1
					ImageURL:              firstImage.ImageURL,
					ImageItemID:           firstImage.ImageItemID,
					SizeImgFlag:           firstImage.SizeImgFlag,
					TransformCVSizeImage:  firstImage.TransformCVSizeImage,
					AISStatus:             firstImage.AISStatus,
					PSTypes:               firstImage.PSTypes,
					MarketingMainImage:    false, // SKU图片不是营销主图
					CommodityCategoryFlag: firstImage.CommodityCategoryFlag,
				},
			},
			OriginalImageInfoList: &[]any{},
		}

		logger.GetGlobalLogger("shein/product").Infof("🔧 自动修复多件商品SKU图片: SKU %s 从SKC复制了图片", sku.SupplierSKU)
		fixedCount++
	}

	if fixedCount > 0 {
		logger.GetGlobalLogger("shein/product").Infof("✅ SKC自动修复完成: 修复了%d个多件商品SKU图片", fixedCount)
	}
}
