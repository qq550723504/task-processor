// Package modules 提供SHEIN平台销售属性的智能筛选功能
package modules

import (
	"strings"
	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/shein/api/attribute"

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
	ctx *TaskContext,
	attributeTemplates *attribute.AttributeTemplateInfo,
) []attribute.AttributeInfo {

	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		logrus.Warn("属性模板为空，无法进行筛选")
		return nil
	}

	var relevantAttributes []attribute.AttributeInfo

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

	logrus.Infof("📊 筛选结果: 从 %d 个销售属性中筛选出 %d 个相关属性",
		f.countSaleAttributes(attributeTemplates.Data[0].AttributeInfos),
		len(relevantAttributes))

	return relevantAttributes
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
func (f *SaleAttributeSmartFilter) analyzeProductVariations(ctx *TaskContext) ProductVariationAnalysis {
	analysis := ProductVariationAnalysis{}

	// 如果是单变体产品，直接返回无变化
	if ctx.Variants == nil || len(*ctx.Variants) <= 1 {
		logrus.Info("单变体产品，无需分析变化")
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
