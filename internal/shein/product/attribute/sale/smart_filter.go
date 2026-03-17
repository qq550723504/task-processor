// Package sale 提供SHEIN平台销售属性的智能筛选功能
package sale

import (
	"strings"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"

	"github.com/sirupsen/logrus"
)

// SaleAttributeSmartFilter 销售属性智能筛选器
type SaleAttributeSmartFilter struct{}

// NewSaleAttributeSmartFilter 创建销售属性智能筛选器实例
func NewSaleAttributeSmartFilter() *SaleAttributeSmartFilter {
	return &SaleAttributeSmartFilter{}
}

// FilterRelevantAttributes 筛选与实际产品数据相关的销售属性
func (f *SaleAttributeSmartFilter) FilterRelevantAttributes(
	ctx *shein.TaskContext,
	attributeTemplates *attribute.AttributeTemplateInfo,
) []attribute.AttributeInfo {

	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		logrus.Warn("属性模板为空，无法进行筛选")
		return nil
	}

	var relevantAttributes []attribute.AttributeInfo

	// 第一步：优先添加SHEIN平台的必填销售属性
	requiredSaleAttrs := f.getRequiredSaleAttributes(attributeTemplates.Data[0].AttributeInfos)
	relevantAttributes = append(relevantAttributes, requiredSaleAttrs...)

	logrus.Infof("🎯 SHEIN平台必填销售属性: 找到 %d 个", len(requiredSaleAttrs))
	for _, attr := range requiredSaleAttrs {
		logrus.Infof("  ⭐ 必填销售属性: %s (ID:%d, Label:%d)", attr.AttributeNameEn, attr.AttributeID, attr.AttributeLabel)
	}

	// 第二步：如果没有必填销售属性，则基于产品变化进行智能筛选
	if len(relevantAttributes) == 0 {
		logrus.Info("🔍 没有找到必填销售属性，开始基于产品变化进行智能筛选")

		// 分析产品数据中的实际变化
		variationAnalysis := f.analyzeProductVariations(ctx)
		logrus.Infof("🔍 产品变化分析结果: %+v", variationAnalysis)

		// 遍历所有销售属性（AttributeType == 1）
		for _, attr := range attributeTemplates.Data[0].AttributeInfos {
			if attr.AttributeType != 1 { // 只处理销售属性
				continue
			}

			// 检查该属性是否与实际产品变化相关
			if f.isAttributeRelevant(attr, variationAnalysis) {
				relevantAttributes = append(relevantAttributes, attr)
				logrus.Infof("✅ 属性 %s (ID:%d) 与产品变化相关，已包含",
					attr.AttributeNameEn, attr.AttributeID)
			} else {
				logrus.Infof("❌ 属性 %s (ID:%d) 与产品变化无关，已过滤",
					attr.AttributeNameEn, attr.AttributeID)
			}
		}
	}

	logrus.Infof("📊 筛选结果: 从 %d 个销售属性中筛选出 %d 个相关属性",
		f.countSaleAttributes(attributeTemplates.Data[0].AttributeInfos),
		len(relevantAttributes))

	// 确保至少有一个销售规格属性
	if len(relevantAttributes) == 0 {
		logrus.Warn("⚠️ 没有筛选出任何销售属性，将选择一个默认属性")
		defaultAttr := f.selectDefaultSaleAttribute(attributeTemplates.Data[0].AttributeInfos)
		if defaultAttr != nil {
			relevantAttributes = append(relevantAttributes, *defaultAttr)
			logrus.Infof("✅ 已添加默认销售属性: %s (ID:%d)", defaultAttr.AttributeNameEn, defaultAttr.AttributeID)
		}
	}

	return relevantAttributes
}

// getRequiredSaleAttributes 获取SHEIN平台的必填销售属性
func (f *SaleAttributeSmartFilter) getRequiredSaleAttributes(attributes []attribute.AttributeInfo) []attribute.AttributeInfo {
	var requiredSaleAttrs []attribute.AttributeInfo

	for _, attr := range attributes {
		// 只处理销售属性（AttributeType == 1）且必填（AttributeLabel == 1）
		if attr.AttributeType == 1 && attr.AttributeLabel == 1 {
			requiredSaleAttrs = append(requiredSaleAttrs, attr)
			logrus.Infof("🎯 发现必填销售属性: %s (ID:%d, Type:%d, Label:%d)",
				attr.AttributeNameEn, attr.AttributeID, attr.AttributeType, attr.AttributeLabel)
		}
	}

	return requiredSaleAttrs
}

// ProductVariationAnalysis 产品变化分析结果
type ProductVariationAnalysis struct {
	HasSizeVariation     bool     // 是否有尺寸变化
	HasColorVariation    bool     // 是否有颜色变化
	HasPatternVariation  bool     // 是否有图案变化
	HasQuantityVariation bool     // 是否有数量变化
	UniqueColors         []string // 唯一颜色列表
	UniqueSizes          []string // 唯一尺寸列表
	UniquePatterns       []string // 唯一图案列表
	UniqueQuantities     []string // 唯一数量列表
}

// analyzeProductVariations 分析产品变化
func (f *SaleAttributeSmartFilter) analyzeProductVariations(ctx *shein.TaskContext) ProductVariationAnalysis {
	analysis := ProductVariationAnalysis{}

	// 如果是单变体产品，仍需要分析基础属性信息
	if ctx.Variants == nil || len(*ctx.Variants) <= 1 {
		logrus.Info("单变体产品，分析基础属性信息")
		// 对于单变体产品，从主产品中提取基础属性信息
		if ctx.AmazonProduct != nil {
			f.extractBasicAttributesFromSingleVariant(ctx.AmazonProduct, &analysis)
		}
		return analysis
	}

	colorSet := make(map[string]bool)
	sizeSet := make(map[string]bool)
	patternSet := make(map[string]bool)
	quantitySet := make(map[string]bool)

	// 分析Amazon产品的Variations数据
	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.Variations) > 0 {
		for _, variation := range ctx.AmazonProduct.Variations {
			f.extractVariationValues(variation, colorSet, sizeSet, patternSet, quantitySet)
		}
	}

	// 分析VariationsValues数据
	if ctx.AmazonProduct != nil && len(ctx.AmazonProduct.VariationsValues) > 0 {
		for _, vv := range ctx.AmazonProduct.VariationsValues {
			f.extractVariationValuesFromList(vv, colorSet, sizeSet, patternSet, quantitySet)
		}
	}

	// 统计结果
	analysis.HasColorVariation = len(colorSet) > 1
	analysis.HasSizeVariation = len(sizeSet) > 1
	analysis.HasPatternVariation = len(patternSet) > 1
	analysis.HasQuantityVariation = len(quantitySet) > 1

	// 转换为切片
	for color := range colorSet {
		analysis.UniqueColors = append(analysis.UniqueColors, color)
	}
	for size := range sizeSet {
		analysis.UniqueSizes = append(analysis.UniqueSizes, size)
	}
	for pattern := range patternSet {
		analysis.UniquePatterns = append(analysis.UniquePatterns, pattern)
	}
	for quantity := range quantitySet {
		analysis.UniqueQuantities = append(analysis.UniqueQuantities, quantity)
	}

	return analysis
}

// extractBasicAttributesFromSingleVariant 从单变体产品中提取基础属性信息
func (f *SaleAttributeSmartFilter) extractBasicAttributesFromSingleVariant(
	product *model.Product,
	analysis *ProductVariationAnalysis,
) {
	// 从产品标题和描述中推断可能的属性
	title := strings.ToLower(product.Title)

	// 检查标题中是否包含颜色、尺寸等关键词
	colorKeywords := []string{"black", "white", "red", "blue", "green", "yellow", "pink", "purple", "gray", "brown", "orange"}
	sizeKeywords := []string{"small", "medium", "large", "xl", "xxl", "xs", "s", "m", "l"}

	for _, color := range colorKeywords {
		if strings.Contains(title, color) {
			analysis.UniqueColors = append(analysis.UniqueColors, color)
			analysis.HasColorVariation = true
			logrus.Infof("🎨 从产品标题中检测到颜色: %s", color)
			break // 只取第一个匹配的颜色
		}
	}

	for _, size := range sizeKeywords {
		if strings.Contains(title, size) {
			analysis.UniqueSizes = append(analysis.UniqueSizes, size)
			analysis.HasSizeVariation = true
			logrus.Infof("📏 从产品标题中检测到尺寸: %s", size)
			break // 只取第一个匹配的尺寸
		}
	}

	// 如果从标题中没有检测到明显的属性，则标记为需要基础属性
	if !analysis.HasColorVariation && !analysis.HasSizeVariation {
		// 对于单变体产品，我们假设它至少需要颜色或尺寸属性之一
		analysis.HasColorVariation = true // 默认认为需要颜色属性
		logrus.Info("🔧 单变体产品默认需要颜色属性")
	}
}

// extractVariationValues 从Variation中提取变化值
func (f *SaleAttributeSmartFilter) extractVariationValues(
	variation model.Variation,
	colorSet, sizeSet, patternSet, quantitySet map[string]bool,
) {
	for key, value := range variation.Attributes {
		keyLower := strings.ToLower(key)

		// 类型断言：将interface{}转换为string
		valueStr, ok := value.(string)
		if !ok {
			continue // 如果不是string类型，跳过
		}

		valueTrimmed := strings.TrimSpace(valueStr)
		if valueTrimmed == "" {
			continue
		}

		switch {
		case strings.Contains(keyLower, "color") || strings.Contains(keyLower, "colour"):
			colorSet[valueTrimmed] = true
		case strings.Contains(keyLower, "size"):
			sizeSet[valueTrimmed] = true
		case strings.Contains(keyLower, "pattern") || strings.Contains(keyLower, "style"):
			patternSet[valueTrimmed] = true
		case strings.Contains(keyLower, "quantity") || strings.Contains(keyLower, "count"):
			quantitySet[valueTrimmed] = true
		}
	}
}

// extractVariationValuesFromList 从VariationValue列表中提取变化值
func (f *SaleAttributeSmartFilter) extractVariationValuesFromList(
	vv model.VariationValue,
	colorSet, sizeSet, patternSet, quantitySet map[string]bool,
) {
	variantNameLower := strings.ToLower(vv.VariantName)

	for _, value := range vv.Values {
		valueTrimmed := strings.TrimSpace(value)
		if valueTrimmed == "" {
			continue
		}

		switch {
		case strings.Contains(variantNameLower, "color") || strings.Contains(variantNameLower, "colour"):
			colorSet[valueTrimmed] = true
		case strings.Contains(variantNameLower, "size"):
			sizeSet[valueTrimmed] = true
		case strings.Contains(variantNameLower, "pattern") || strings.Contains(variantNameLower, "style"):
			patternSet[valueTrimmed] = true
		case strings.Contains(variantNameLower, "quantity") || strings.Contains(variantNameLower, "count"):
			quantitySet[valueTrimmed] = true
		}
	}
}

// isAttributeRelevant 判断属性是否与产品变化相关
func (f *SaleAttributeSmartFilter) isAttributeRelevant(
	attr attribute.AttributeInfo,
	analysis ProductVariationAnalysis,
) bool {
	// 必填属性(attribute_label=1)总是相关的，但这里不应该被调用到
	// 因为必填属性已经在FilterRelevantAttributes的第一步中处理了
	if attr.AttributeLabel == 1 {
		logrus.Infof("⭐ 属性 %s (ID:%d) 是必填属性，应该已在第一步处理", attr.AttributeNameEn, attr.AttributeID)
		return true
	}

	attrNameLower := strings.ToLower(attr.AttributeNameEn)

	// 基于属性名称和实际变化进行匹配
	switch {
	case strings.Contains(attrNameLower, "color") || strings.Contains(attrNameLower, "colour"):
		return analysis.HasColorVariation
	case strings.Contains(attrNameLower, "size"):
		return analysis.HasSizeVariation
	case strings.Contains(attrNameLower, "pattern") || strings.Contains(attrNameLower, "style"):
		return analysis.HasPatternVariation
	case strings.Contains(attrNameLower, "quantity") || strings.Contains(attrNameLower, "count"):
		return analysis.HasQuantityVariation
	default:
		// 对于其他属性，如果有多个变体且属性有多个值，则认为相关
		return len(analysis.UniqueColors) > 1 || len(analysis.UniqueSizes) > 1
	}
}

// countSaleAttributes 统计销售属性数量
func (f *SaleAttributeSmartFilter) countSaleAttributes(attributes []attribute.AttributeInfo) int {
	count := 0
	for _, attr := range attributes {
		if attr.AttributeType == 1 {
			count++
		}
	}
	return count
}

// selectDefaultSaleAttribute 选择一个默认的销售属性
func (f *SaleAttributeSmartFilter) selectDefaultSaleAttribute(attributes []attribute.AttributeInfo) *attribute.AttributeInfo {
	// 优先级顺序：必填属性(attribute_label=1) > 颜色 > 尺寸 > 其他销售属性
	var requiredAttr, colorAttr, sizeAttr, otherSaleAttr *attribute.AttributeInfo

	for _, attr := range attributes {
		if attr.AttributeType != 1 { // 只处理销售属性
			continue
		}

		attrNameLower := strings.ToLower(attr.AttributeNameEn)

		// 首先检查是否为必填属性
		if attr.AttributeLabel == 1 {
			if requiredAttr == nil {
				requiredAttr = &attr
			}
			continue
		}

		// 然后按类型分类
		switch {
		case strings.Contains(attrNameLower, "color") || strings.Contains(attrNameLower, "colour"):
			if colorAttr == nil {
				colorAttr = &attr
			}
		case strings.Contains(attrNameLower, "size"):
			if sizeAttr == nil {
				sizeAttr = &attr
			}
		default:
			if otherSaleAttr == nil {
				otherSaleAttr = &attr
			}
		}
	}

	// 按优先级返回
	if requiredAttr != nil {
		logrus.Infof("⭐ 选择必填属性作为默认销售属性: %s (label=%d)", requiredAttr.AttributeNameEn, requiredAttr.AttributeLabel)
		return requiredAttr
	}
	if colorAttr != nil {
		logrus.Infof("🎨 选择颜色属性作为默认销售属性: %s", colorAttr.AttributeNameEn)
		return colorAttr
	}
	if sizeAttr != nil {
		logrus.Infof("📏 选择尺寸属性作为默认销售属性: %s", sizeAttr.AttributeNameEn)
		return sizeAttr
	}
	if otherSaleAttr != nil {
		logrus.Infof("🔧 选择其他销售属性作为默认: %s", otherSaleAttr.AttributeNameEn)
		return otherSaleAttr
	}

	logrus.Warn("❌ 未找到任何销售属性")
	return nil
}

