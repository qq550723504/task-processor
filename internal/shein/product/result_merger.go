// Package product 提供SHEIN平台结果合并功能
package product

import (
	"task-processor/internal/core/logger"
	"fmt"
	"task-processor/internal/shein"
	sheinattr "task-processor/internal/shein/product/attribute"

)

// ResultMerger 结果合并器，负责合并多个批次的处理结果
type ResultMerger struct{}

// NewResultMerger 创建新的结果合并器
func NewResultMerger() *ResultMerger {
	return &ResultMerger{}
}

// MergeResults 合并多个批次的处理结果
func (m *ResultMerger) MergeResults(results []sheinattr.ResultSaleAttribute) sheinattr.ResultSaleAttribute {
	if len(results) == 0 {
		return sheinattr.ResultSaleAttribute{}
	}

	if len(results) == 1 {
		return results[0]
	}

	logger.GetGlobalLogger("shein/product").Infof("🔄 开始合并%d个批次结果", len(results))

	// 合并销售属性
	mergedSaleAttributes := m.mergeSaleAttributes(results)

	// 合并变体
	mergedVariants := m.mergeVariants(results)

	result := sheinattr.ResultSaleAttribute{
		SaleAttributes: mergedSaleAttributes,
		Variants:       mergedVariants,
	}

	logger.GetGlobalLogger("shein/product").Infof("✅ 结果合并完成: 销售属性=%d个, 变体=%d个",
		len(result.SaleAttributes), len(result.Variants))

	return result
}

// mergeSaleAttributes 合并销售属性
func (m *ResultMerger) mergeSaleAttributes(results []sheinattr.ResultSaleAttribute) []sheinattr.SaleAttribute {
	// 使用map去重，key为属性ID
	attrMap := make(map[int]*sheinattr.SaleAttribute)

	for _, result := range results {
		for _, attr := range result.SaleAttributes {
			if existingAttr, exists := attrMap[attr.AttrID]; exists {
				// 合并属性值
				existingAttr.AttrValue = m.mergeAttributeValues(existingAttr.AttrValue, attr.AttrValue)
			} else {
				// 复制属性
				newAttr := sheinattr.SaleAttribute{
					AttrID:    attr.AttrID,
					AttrValue: make([]sheinattr.AttributeValue, len(attr.AttrValue)),
				}
				copy(newAttr.AttrValue, attr.AttrValue)
				attrMap[attr.AttrID] = &newAttr
			}
		}
	}

	// 转换为切片
	var mergedAttrs []sheinattr.SaleAttribute
	for _, attr := range attrMap {
		mergedAttrs = append(mergedAttrs, *attr)
	}

	return mergedAttrs
}

// mergeAttributeValues 合并属性值，去重
func (m *ResultMerger) mergeAttributeValues(existing, new []sheinattr.AttributeValue) []sheinattr.AttributeValue {
	// 使用map去重，key为属性值的value
	valueMap := make(map[string]sheinattr.AttributeValue)

	// 添加现有值
	for _, val := range existing {
		valueMap[val.Value] = val
	}

	// 添加新值
	for _, val := range new {
		if _, exists := valueMap[val.Value]; !exists {
			valueMap[val.Value] = val
		}
	}

	// 转换为切片
	var merged []sheinattr.AttributeValue
	for _, val := range valueMap {
		merged = append(merged, val)
	}

	return merged
}

// mergeVariants 合并变体
func (m *ResultMerger) mergeVariants(results []sheinattr.ResultSaleAttribute) []shein.Variant {
	var allVariants []shein.Variant

	for _, result := range results {
		allVariants = append(allVariants, result.Variants...)
	}

	// 去重（基于ASIN）
	variantMap := make(map[string]shein.Variant)
	for _, variant := range allVariants {
		if variant.ASIN != "" {
			variantMap[variant.ASIN] = variant
		}
	}

	// 转换为切片
	var mergedVariants []shein.Variant
	for _, variant := range variantMap {
		mergedVariants = append(mergedVariants, variant)
	}

	return mergedVariants
}

// ValidateMergedResult 验证合并后的结果
func (m *ResultMerger) ValidateMergedResult(result sheinattr.ResultSaleAttribute) []string {
	var issues []string

	// 检查销售属性
	if len(result.SaleAttributes) == 0 {
		issues = append(issues, "没有销售属性")
	}

	// 检查变体
	if len(result.Variants) == 0 {
		issues = append(issues, "没有变体")
	}

	// 检查变体属性完整性
	for i, variant := range result.Variants {
		if len(variant.Attributes) == 0 {
			issues = append(issues, fmt.Sprintf("变体%d缺少属性", i))
		}

		if variant.ASIN == "" {
			issues = append(issues, fmt.Sprintf("变体%d缺少ASIN", i))
		}
	}

	// 检查属性值一致性
	for _, attr := range result.SaleAttributes {
		if len(attr.AttrValue) == 0 {
			issues = append(issues, fmt.Sprintf("属性%d没有属性值", attr.AttrID))
		}
	}

	if len(issues) > 0 {
		logger.GetGlobalLogger("shein/product").Warnf("⚠️ 合并结果验证发现问题: %v", issues)
	}

	return issues
}
