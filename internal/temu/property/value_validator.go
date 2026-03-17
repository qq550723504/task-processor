// Package property 提供TEMU平台的属性值验证功能
package property

import (
	"fmt"

	models "task-processor/internal/temu/api/product"
	temutemplate "task-processor/internal/temu/api/template"

	"github.com/sirupsen/logrus"
)

// PropertyValueValidator 属性值验证器 - 专门验证属性值的有效性
type PropertyValueValidator struct {
	logger *logrus.Entry
}

// NewPropertyValueValidator 创建新的属性值验证器
func NewPropertyValueValidator(logger *logrus.Entry) *PropertyValueValidator {
	return &PropertyValueValidator{
		logger: logger,
	}
}

// ValidateSelectionValue 验证选择类型属性值是否有效（包含条件依赖验证）
// 返回: (isValid bool, validVID int, validValue string, error)
func (v *PropertyValueValidator) ValidateSelectionValue(
	prop models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
) (bool, int, string, error) {

	v.logger.Debugf("🔍 验证选择类型属性: PID=%d, Value='%s', VID=%d",
		prop.Pid, prop.Value, prop.Vid)

	// 检查是否有可选值列表
	if len(templateProp.Values) == 0 {
		return false, 0, "", fmt.Errorf("属性 %s (PID=%d) 没有可选值列表",
			templateProp.Name, templateProp.PID)
	}

	// 1. 如果有VID，验证VID是否有效
	if prop.Vid != 0 {
		for _, validValue := range templateProp.Values {
			if validValue.VID == prop.Vid {
				v.logger.Debugf("✅ VID验证通过: VID=%d, Value='%s'",
					validValue.VID, validValue.Value)
				return true, validValue.VID, validValue.Value, nil
			}
		}
		v.logger.Warnf("⚠️ 无效的VID: %d，属性 %s", prop.Vid, templateProp.Name)
	}

	// 2. 如果有Value，验证Value是否在可选列表中
	if prop.Value != "" {
		for _, validValue := range templateProp.Values {
			if validValue.Value == prop.Value {
				v.logger.Debugf("✅ Value验证通过: Value='%s', VID=%d",
					validValue.Value, validValue.VID)
				return true, validValue.VID, validValue.Value, nil
			}
		}
		v.logger.Warnf("⚠️ 无效的Value: '%s'，属性 %s", prop.Value, templateProp.Name)
	}

	// 3. 都无效，返回false
	v.logger.Errorf("❌ 属性值验证失败: PID=%d, Value='%s', VID=%d",
		prop.Pid, prop.Value, prop.Vid)
	return false, 0, "", fmt.Errorf("属性值无效")
}

// ValidateSelectionValueWithDependency 验证选择类型属性值（包含条件依赖验证）
// 参数:
//   - prop: 要验证的属性
//   - templateProp: 模板属性定义
//   - parentProperties: 已填充的父属性列表（用于条件依赖验证）
//
// 返回: (isValid bool, validVID int, validValue string, error)
func (v *PropertyValueValidator) ValidateSelectionValueWithDependency(
	prop models.PropertyItem,
	templateProp temutemplate.TemplateRespGoodsProperty,
	parentProperties []models.PropertyItem,
) (bool, int, string, error) {

	v.logger.Debugf("🔍 验证条件依赖属性: PID=%d, Value='%s', VID=%d",
		prop.Pid, prop.Value, prop.Vid)

	// 检查是否有可选值列表
	if len(templateProp.Values) == 0 {
		return false, 0, "", fmt.Errorf("属性 %s (PID=%d) 没有可选值列表",
			templateProp.Name, templateProp.PID)
	}

	// 获取有效的属性值列表（考虑条件依赖）
	validValues := v.getValidValuesWithDependency(templateProp, parentProperties)
	if len(validValues) == 0 {
		return false, 0, "", fmt.Errorf("属性 %s (PID=%d) 在当前条件下没有有效值",
			templateProp.Name, templateProp.PID)
	}

	// 1. 如果有VID，验证VID是否在有效列表中
	if prop.Vid != 0 {
		for _, validValue := range validValues {
			if validValue.VID == prop.Vid {
				v.logger.Debugf("✅ 条件依赖VID验证通过: VID=%d, Value='%s'",
					validValue.VID, validValue.Value)
				return true, validValue.VID, validValue.Value, nil
			}
		}
		v.logger.Warnf("⚠️ VID %d 不满足条件依赖约束，属性 %s", prop.Vid, templateProp.Name)
	}

	// 2. 如果有Value，验证Value是否在有效列表中
	if prop.Value != "" {
		for _, validValue := range validValues {
			if validValue.Value == prop.Value {
				v.logger.Debugf("✅ 条件依赖Value验证通过: Value='%s', VID=%d",
					validValue.Value, validValue.VID)
				return true, validValue.VID, validValue.Value, nil
			}
		}
		v.logger.Warnf("⚠️ Value '%s' 不满足条件依赖约束，属性 %s", prop.Value, templateProp.Name)
	}

	// 3. 都无效，返回false
	v.logger.Errorf("❌ 条件依赖属性值验证失败: PID=%d, Value='%s', VID=%d",
		prop.Pid, prop.Value, prop.Vid)
	return false, 0, "", fmt.Errorf("属性值不满足条件依赖约束")
}

// getValidValuesWithDependency 获取考虑条件依赖的有效值列表
func (v *PropertyValueValidator) getValidValuesWithDependency(
	templateProp temutemplate.TemplateRespGoodsProperty,
	parentProperties []models.PropertyItem,
) []temutemplate.PropertyValue {

	// 如果没有条件依赖，返回所有值
	if len(templateProp.TemplatePropertyValueParentList) == 0 {
		return templateProp.Values
	}

	// 创建父属性VID映射
	parentVIDMap := make(map[int]bool)
	for _, parentProp := range parentProperties {
		if parentProp.Vid != 0 {
			parentVIDMap[parentProp.Vid] = true
		}
	}

	v.logger.Debugf("🔍 父属性VID列表: %v", getMapKeys(parentVIDMap))

	// 收集满足条件的有效值
	var validValues []temutemplate.PropertyValue

	for _, value := range templateProp.Values {
		// 如果值没有父VID约束，直接添加
		if len(value.ParentVIDs) == 0 {
			validValues = append(validValues, value)
			continue
		}

		// 检查是否有父VID满足条件
		hasValidParent := false
		for _, parentVID := range value.ParentVIDs {
			if parentVIDMap[parentVID] {
				hasValidParent = true
				v.logger.Debugf("✅ 值 '%s' (VID=%d) 满足父VID约束: %d",
					value.Value, value.VID, parentVID)
				break
			}
		}

		if hasValidParent {
			validValues = append(validValues, value)
		} else {
			v.logger.Debugf("❌ 值 '%s' (VID=%d) 不满足父VID约束: %v",
				value.Value, value.VID, value.ParentVIDs)
		}
	}

	v.logger.Debugf("🔍 条件依赖过滤结果: 原始值=%d, 有效值=%d",
		len(templateProp.Values), len(validValues))

	return validValues
}

// getMapKeys 获取map的所有key
func getMapKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// IsValueInValidList 检查值是否在有效列表中
func (v *PropertyValueValidator) IsValueInValidList(value string, validValues []temutemplate.PropertyValue) bool {
	for _, validValue := range validValues {
		if validValue.Value == value {
			return true
		}
	}
	return false
}

// IsVIDInValidList 检查VID是否在有效列表中
func (v *PropertyValueValidator) IsVIDInValidList(vid int, validValues []temutemplate.PropertyValue) bool {
	for _, validValue := range validValues {
		if validValue.VID == vid {
			return true
		}
	}
	return false
}

// GetValidValueByVID 根据VID获取有效值
func (v *PropertyValueValidator) GetValidValueByVID(vid int, validValues []temutemplate.PropertyValue) *temutemplate.PropertyValue {
	for _, validValue := range validValues {
		if validValue.VID == vid {
			return &validValue
		}
	}
	return nil
}

// GetValidValueByValue 根据Value获取有效值
func (v *PropertyValueValidator) GetValidValueByValue(value string, validValues []temutemplate.PropertyValue) *temutemplate.PropertyValue {
	for _, validValue := range validValues {
		if validValue.Value == value {
			return &validValue
		}
	}
	return nil
}

// ValidateAllSelectionProperties 批量验证所有选择类型属性
func (v *PropertyValueValidator) ValidateAllSelectionProperties(
	properties []models.PropertyItem,
	templateProps []temutemplate.TemplateRespGoodsProperty,
) []ValidationResult {

	results := make([]ValidationResult, 0, len(properties))

	// 创建模板属性映射
	templateMap := make(map[int]temutemplate.TemplateRespGoodsProperty)
	for _, tmpl := range templateProps {
		templateMap[tmpl.PID] = tmpl
	}

	// 验证每个属性
	for _, prop := range properties {
		templateProp, exists := templateMap[prop.Pid]
		if !exists {
			continue
		}

		// 只验证选择类型属性
		if templateProp.PropertyValueType != 1 {
			continue
		}

		isValid, validVID, validValue, err := v.ValidateSelectionValue(prop, templateProp)

		result := ValidationResult{
			Property:     prop,
			TemplateProp: templateProp,
			IsValid:      isValid,
			ValidVID:     validVID,
			ValidValue:   validValue,
			Error:        err,
		}

		results = append(results, result)
	}

	return results
}

// ValidationResult 验证结果
type ValidationResult struct {
	Property     models.PropertyItem
	TemplateProp temutemplate.TemplateRespGoodsProperty
	IsValid      bool
	ValidVID     int
	ValidValue   string
	Error        error
}

// GetValidValuesWithDependency 获取考虑条件依赖的有效值列表（公开方法）
func (v *PropertyValueValidator) GetValidValuesWithDependency(
	templateProp temutemplate.TemplateRespGoodsProperty,
	parentProperties []models.PropertyItem,
) []temutemplate.PropertyValue {
	return v.getValidValuesWithDependency(templateProp, parentProperties)
}
