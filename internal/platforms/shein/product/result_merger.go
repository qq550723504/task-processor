// Package product 提供SHEIN平台结果合并功能
package product

import (
	"fmt"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// ResultMerger 结果合并器，负责合并多个批次的处理结果
type ResultMerger struct{}

// NewResultMerger 创建新的结果合并器
func NewResultMerger() *ResultMerger {
	return &ResultMerger{}
}

// MergeResults 合并多个批次的处理结果
func (m *ResultMerger) MergeResults(results []model.ResultSaleAttribute) model.ResultSaleAttribute {
	if len(results) == 0 {
		return model.ResultSaleAttribute{}
	}

	if len(results) == 1 {
		return results[0]
	}

	logrus.Infof("🔄 开始合并%d个批次结果", len(results))

	// 合并销售属性
	mergedSaleAttributes := m.mergeSaleAttributes(results)

	// 合并变体
	mergedVariants := m.mergeVariants(results)

	result := model.ResultSaleAttribute{
		SaleAttributes: mergedSaleAttributes,
		Variants:       mergedVariants,
	}

	logrus.Infof("✅ 结果合并完成: 销售属性=%d个, 变体=%d个",
		len(result.SaleAttributes), len(result.Variants))

	return result
}

// mergeSaleAttributes 合并销售属性
func (m *ResultMerger) mergeSaleAttributes(results []model.ResultSaleAttribute) []model.SaleAttribute {
	// 使用map去重，key为属性ID
	attrMap := make(map[int]*model.SaleAttribute)

	for _, result := range results {
		for _, attr := range result.SaleAttributes {
			if existingAttr, exists := attrMap[attr.AttrID]; exists {
				// 合并属性值
				existingAttr.AttrValue = m.mergeAttributeValues(existingAttr.AttrValue, attr.AttrValue)
			} else {
				// 复制属性
				newAttr := model.SaleAttribute{
					AttrID:    attr.AttrID,
					AttrValue: make([]model.AttributeValue, len(attr.AttrValue)),
				}
				copy(newAttr.AttrValue, attr.AttrValue)
				attrMap[attr.AttrID] = &newAttr
			}
		}
	}

	// 转换为切片
	var mergedAttrs []model.SaleAttribute
	for _, attr := range attrMap {
		mergedAttrs = append(mergedAttrs, *attr)
	}

	return mergedAttrs
}

// mergeAttributeValues 合并属性值，去重
func (m *ResultMerger) mergeAttributeValues(existing, new []model.AttributeValue) []model.AttributeValue {
	// 使用map去重，key为属性值的value
	valueMap := make(map[string]model.AttributeValue)

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
	var merged []model.AttributeValue
	for _, val := range valueMap {
		merged = append(merged, val)
	}

	return merged
}

// mergeVariants 合并变体
func (m *ResultMerger) mergeVariants(results []model.ResultSaleAttribute) []model.Variant {
	var allVariants []model.Variant

	for _, result := range results {
		allVariants = append(allVariants, result.Variants...)
	}

	// 去重（基于ASIN）
	variantMap := make(map[string]model.Variant)
	for _, variant := range allVariants {
		if variant.ASIN != "" {
			variantMap[variant.ASIN] = variant
		}
	}

	// 转换为切片
	var mergedVariants []model.Variant
	for _, variant := range variantMap {
		mergedVariants = append(mergedVariants, variant)
	}

	return mergedVariants
}

// ValidateMergedResult 验证合并后的结果
func (m *ResultMerger) ValidateMergedResult(result model.ResultSaleAttribute) []string {
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
		logrus.Warnf("⚠️ 合并结果验证发现问题: %v", issues)
	}

	return issues
}
