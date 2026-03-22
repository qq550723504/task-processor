// Package skc 提供SHEIN平台SKC验证和工具功能
package skc

import (
	"task-processor/internal/core/logger"
	"fmt"
	"strings"
	"task-processor/internal/shein"

)

// SKCValidationUtils SKC验证工具
type SKCValidationUtils struct {
	taskContext *shein.TaskContext
}

// NewSKCValidationUtils 创建新的SKC验证工具
func NewSKCValidationUtils(taskContext *shein.TaskContext) *SKCValidationUtils {
	return &SKCValidationUtils{
		taskContext: taskContext,
	}
}

// ValidateAttributeStrategy 验证属性策略的有效性
func (v *SKCValidationUtils) ValidateAttributeStrategy(strategy shein.AttributeStrategy, saleAttributeData shein.ResultSaleAttribute) error {
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

		logger.GetGlobalLogger("shein/product").Infof("次要属性验证结果: 属性ID=%d, 匹配率=%.1f%% (%d/%d)",
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
		warnings = append(warnings, "没有有效的变体数据）")
	} else if float64(validVariantCount)/float64(len(saleAttributeData.Variants)) < 0.5 {
		warnings = append(warnings, fmt.Sprintf("有效变体比例过低: %.1f%% (%d/%d)",
			float64(validVariantCount)*100/float64(len(saleAttributeData.Variants)),
			validVariantCount, len(saleAttributeData.Variants)))
	}

	if len(warnings) > 0 {
		return fmt.Errorf("策略验证发现问题: %s", strings.Join(warnings, "; "))
	}

	logger.GetGlobalLogger("shein/product").Infof("属性策略验证通过: 策略=%s, 有效变体=%d/%d",
		strategy.StrategyType, validVariantCount, len(saleAttributeData.Variants))
	return nil
}
