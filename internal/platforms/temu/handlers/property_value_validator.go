// Package handlers 提供TEMU平台的属性值验证功能
package handlers

import (
	"fmt"

	"task-processor/internal/platforms/temu/types"

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

// ValidateSelectionValue 验证选择类型属性值是否有效
// 返回: (isValid bool, validVID int, validValue string, error)
func (v *PropertyValueValidator) ValidateSelectionValue(
	prop types.PropertyItem,
	templateProp types.TemplateRespGoodsProperty,
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

// IsValueInValidList 检查值是否在有效列表中
func (v *PropertyValueValidator) IsValueInValidList(value string, validValues []types.PropertyValue) bool {
	for _, validValue := range validValues {
		if validValue.Value == value {
			return true
		}
	}
	return false
}

// IsVIDInValidList 检查VID是否在有效列表中
func (v *PropertyValueValidator) IsVIDInValidList(vid int, validValues []types.PropertyValue) bool {
	for _, validValue := range validValues {
		if validValue.VID == vid {
			return true
		}
	}
	return false
}

// GetValidValueByVID 根据VID获取有效值
func (v *PropertyValueValidator) GetValidValueByVID(vid int, validValues []types.PropertyValue) *types.PropertyValue {
	for _, validValue := range validValues {
		if validValue.VID == vid {
			return &validValue
		}
	}
	return nil
}

// GetValidValueByValue 根据Value获取有效值
func (v *PropertyValueValidator) GetValidValueByValue(value string, validValues []types.PropertyValue) *types.PropertyValue {
	for _, validValue := range validValues {
		if validValue.Value == value {
			return &validValue
		}
	}
	return nil
}

// ValidateAllSelectionProperties 批量验证所有选择类型属性
func (v *PropertyValueValidator) ValidateAllSelectionProperties(
	properties []types.PropertyItem,
	templateProps []types.TemplateRespGoodsProperty,
) []ValidationResult {

	results := make([]ValidationResult, 0, len(properties))

	// 创建模板属性映射
	templateMap := make(map[int]types.TemplateRespGoodsProperty)
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
	Property     types.PropertyItem
	TemplateProp types.TemplateRespGoodsProperty
	IsValid      bool
	ValidVID     int
	ValidValue   string
	Error        error
}
