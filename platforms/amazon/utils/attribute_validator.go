package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// AttributeValidator 属性验证器
type AttributeValidator struct {
	mapper *AttributeMapper
}

// NewAttributeValidator 创建属性验证器
func NewAttributeValidator(mapper *AttributeMapper) *AttributeValidator {
	return &AttributeValidator{
		mapper: mapper,
	}
}

// ValidateAttributes 验证属性
func (v *AttributeValidator) ValidateAttributes(
	attributes map[string]interface{},
	productType string,
) error {
	// 获取产品类型配置
	typeConfig, err := v.mapper.GetProductTypeConfig(productType)
	if err != nil {
		return err
	}

	// 验证必填属性
	for _, attrName := range typeConfig.RequiredAttributes {
		value, exists := attributes[attrName]
		if !exists || value == nil {
			return fmt.Errorf("缺少必填属性: %s", attrName)
		}

		if err := v.validateAttribute(attrName, value); err != nil {
			return fmt.Errorf("属性 %s 验证失败: %w", attrName, err)
		}
	}

	// 验证可选属性
	for _, attrName := range typeConfig.OptionalAttributes {
		if value, exists := attributes[attrName]; exists && value != nil {
			if err := v.validateAttribute(attrName, value); err != nil {
				return fmt.Errorf("属性 %s 验证失败: %w", attrName, err)
			}
		}
	}

	return nil
}

// validateAttribute 验证单个属性
func (v *AttributeValidator) validateAttribute(attrName string, value interface{}) error {
	// 获取验证规则
	rule, exists := v.mapper.config.ValidationRules[attrName]
	if !exists {
		return nil // 没有验证规则，跳过
	}

	// 验证字符串类型
	if strVal, ok := value.(string); ok {
		return v.validateStringAttribute(attrName, strVal, rule)
	}

	// 验证数值类型
	if numVal, ok := v.toFloat64(value); ok {
		return v.validateNumericAttribute(attrName, numVal, rule)
	}

	return nil
}

// validateStringAttribute 验证字符串属性
func (v *AttributeValidator) validateStringAttribute(
	attrName string,
	value string,
	rule ValidationRule,
) error {
	value = strings.TrimSpace(value)

	// 验证最小长度
	if rule.MinLength > 0 && len(value) < rule.MinLength {
		return fmt.Errorf("长度不能小于 %d 个字符", rule.MinLength)
	}

	// 验证最大长度
	if rule.MaxLength > 0 && len(value) > rule.MaxLength {
		return fmt.Errorf("长度不能超过 %d 个字符", rule.MaxLength)
	}

	// 验证正则表达式
	if rule.Pattern != "" {
		matched, err := regexp.MatchString(rule.Pattern, value)
		if err != nil {
			return fmt.Errorf("正则表达式验证失败: %w", err)
		}
		if !matched {
			return fmt.Errorf("格式不符合要求")
		}
	}

	// 验证允许值列表
	if len(rule.AllowedValues) > 0 {
		allowed := false
		for _, allowedVal := range rule.AllowedValues {
			if value == allowedVal {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("值必须是以下之一: %v", rule.AllowedValues)
		}
	}

	return nil
}

// validateNumericAttribute 验证数值属性
func (v *AttributeValidator) validateNumericAttribute(
	attrName string,
	value float64,
	rule ValidationRule,
) error {
	// 验证最小值
	if rule.MinValue > 0 && value < rule.MinValue {
		return fmt.Errorf("值不能小于 %.2f", rule.MinValue)
	}

	// 验证最大值
	if rule.MaxValue > 0 && value > rule.MaxValue {
		return fmt.Errorf("值不能超过 %.2f", rule.MaxValue)
	}

	return nil
}

// toFloat64 转换为 float64
func (v *AttributeValidator) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// ValidateRequiredFields 验证必填字段
func (v *AttributeValidator) ValidateRequiredFields(
	attributes map[string]interface{},
	requiredFields []string,
) error {
	for _, field := range requiredFields {
		value, exists := attributes[field]
		if !exists || value == nil {
			return fmt.Errorf("缺少必填字段: %s", field)
		}

		// 验证字符串不为空
		if strVal, ok := value.(string); ok {
			if strings.TrimSpace(strVal) == "" {
				return fmt.Errorf("字段 %s 不能为空", field)
			}
		}
	}
	return nil
}

// ValidateFieldLength 验证字段长度
func (v *AttributeValidator) ValidateFieldLength(
	fieldName string,
	value string,
	maxLength int,
) error {
	value = strings.TrimSpace(value)
	if len(value) > maxLength {
		return fmt.Errorf("字段 %s 长度不能超过 %d 个字符", fieldName, maxLength)
	}
	return nil
}

// ValidateFieldPattern 验证字段格式
func (v *AttributeValidator) ValidateFieldPattern(
	fieldName string,
	value string,
	pattern string,
) error {
	matched, err := regexp.MatchString(pattern, value)
	if err != nil {
		return fmt.Errorf("字段 %s 正则表达式验证失败: %w", fieldName, err)
	}
	if !matched {
		return fmt.Errorf("字段 %s 格式不符合要求", fieldName)
	}
	return nil
}
