package handlers

import (
	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
)

// PropertyValidator 属性验证器
type PropertyValidator struct {
	logger *logrus.Entry
}

// NewPropertyValidator 创建新的属性验证器
func NewPropertyValidator(logger *logrus.Entry) *PropertyValidator {
	return &PropertyValidator{
		logger: logger,
	}
}

// ValidateAndFixProperties 验证和修复属性值
func (v *PropertyValidator) ValidateAndFixProperties(properties []types.PropertyItem, data PropertyMappingData) []types.PropertyItem {
	// 先应用属性关联过滤规则（基于ShowCondition）
	properties = v.filterByPropertyRelations(properties, data)

	// 创建属性映射
	propMap := make(map[int]TemuPropertyOption)
	refPidToPropMap := make(map[int]TemuPropertyOption) // RefPID到属性的映射
	for _, prop := range data.TemuProperties {
		propMap[prop.PID] = prop
		refPidToPropMap[prop.RefPID] = prop
	}

	var validatedProperties []types.PropertyItem
	processedPIDs := make(map[int]bool)
	refPidCounts := make(map[int]int) // 统计每个RefPID的属性项数量（这是关键修复）

	// 调试：记录AI返回的原始属性
	v.logger.Infof("AI返回了 %d 个属性，开始验证", len(properties))
	for i, prop := range properties {
		v.logger.Debugf("AI属性[%d]: PID=%d, RefPid=%d, Value=%s, Vid=%d",
			i, prop.Pid, prop.RefPid, prop.Value, prop.Vid)
	}

	// 预处理：确保所有属性都有正确的RefPid、TemplatePid等字段
	for i := range properties {
		if templateProp, exists := propMap[properties[i].Pid]; exists {
			properties[i].RefPid = templateProp.RefPID
			properties[i].TemplatePid = templateProp.TemplatePID
			properties[i].TemplateModuleID = templateProp.TemplateModuleID
		}
	}

	// 验证AI选择的属性
	for _, prop := range properties {
		templateProp, exists := propMap[prop.Pid]
		if !exists {
			v.logger.Warnf("属性PID %d 不在模板中，跳过", prop.Pid)
			continue
		}

		// 确保使用正确的template_pid和ref_pid
		prop.RefPid = templateProp.RefPID
		prop.TemplatePid = templateProp.TemplatePID
		prop.TemplateModuleID = templateProp.TemplateModuleID

		// 检查选择数量限制（使用RefPID进行统计）
		if templateProp.PropertyValueType == 1 && templateProp.ChooseMaxNum > 0 {
			currentCount := refPidCounts[templateProp.RefPID]
			if currentCount >= templateProp.ChooseMaxNum {
				continue
			}
		}

		if v.isValidPropertyValue(prop, templateProp) {
			validatedProperties = append(validatedProperties, prop)
			processedPIDs[prop.Pid] = true
			refPidCounts[templateProp.RefPID]++
		} else {
			// 属性值无效
			if templateProp.Required {
				// 必填属性：尝试修复
				v.logger.Warnf("⚠️ 必填属性 %s (RefPID=%d) 值无效，尝试修复", templateProp.Name, templateProp.RefPID)
				if fixedProp := v.fixPropertyValue(prop, templateProp); fixedProp != nil {
					// 在修复后也要检查选择数量限制
					if templateProp.PropertyValueType == 1 && templateProp.ChooseMaxNum > 0 {
						if refPidCounts[templateProp.RefPID] >= templateProp.ChooseMaxNum {
							v.logger.Warnf("修复后的属性RefPID %d (PID: %d) 超过最大选择数量限制 %d，跳过",
								templateProp.RefPID, prop.Pid, templateProp.ChooseMaxNum)
							continue
						}
					}
					// 确保修复后的属性也使用正确的template_pid和ref_pid
					fixedProp.RefPid = templateProp.RefPID
					fixedProp.TemplatePid = templateProp.TemplatePID
					fixedProp.TemplateModuleID = templateProp.TemplateModuleID
					validatedProperties = append(validatedProperties, *fixedProp)
					processedPIDs[prop.Pid] = true
					refPidCounts[templateProp.RefPID]++
				} else {
					v.logger.Errorf("❌ 必填属性 %s (RefPID=%d) 修复失败", templateProp.Name, templateProp.RefPID)
				}
			} else {
				// 可选属性：直接跳过，不修复
				v.logger.Infof("⭕ 可选属性 %s (RefPID=%d) 值无效，直接跳过", templateProp.Name, templateProp.RefPID)
			}
		}
	}

	// 添加缺失的必填属性
	for _, templateProp := range data.TemuProperties {
		if templateProp.Required && !processedPIDs[templateProp.PID] {
			if defaultProp := v.createDefaultProperty(templateProp); defaultProp != nil {
				validatedProperties = append(validatedProperties, *defaultProp)
				v.logger.Infof("添加缺失的必填属性: PID=%d, RefPID=%d, 值=%s",
					templateProp.PID, templateProp.RefPID, defaultProp.Value)
			}
		}
	}

	// 最终去重：确保单选属性的RefPID只出现一次
	finalProperties := v.deduplicateProperties(validatedProperties, data.TemuProperties)

	v.logger.Infof("属性验证完成: 原始=%d, 去重后=%d", len(validatedProperties), len(finalProperties))
	return finalProperties
}
