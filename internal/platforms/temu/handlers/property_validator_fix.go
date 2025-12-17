package handlers

import (
	"strings"

	"task-processor/internal/platforms/temu/types"
)

// isValidPropertyValue 验证属性值是否有效
func (v *PropertyValidator) isValidPropertyValue(prop types.PropertyItem, templateProp TemuPropertyOption) bool {
	switch templateProp.PropertyValueType {
	case 1: // 选择类型
		// vid为0表示无效值，必须从可选值中选择
		if prop.Vid == 0 {
			v.logger.Warnf("❌ 选择类型属性 %s (RefPID=%d) 的vid为0，这是无效值",
				templateProp.Name, templateProp.RefPID)
			return false
		}
		// 验证vid是否在可选值列表中
		for _, value := range templateProp.Values {
			if value.VID == prop.Vid {
				return true
			}
		}
		v.logger.Warnf("❌ 选择类型属性 %s (RefPID=%d) 的vid=%d不在可选值列表中",
			templateProp.Name, templateProp.RefPID, prop.Vid)
		return false
	case 2, 3: // 数字类型或文本类型
		return prop.Value != ""
	default:
		return prop.Value != ""
	}
}

// fixPropertyValue 尝试修复属性值
func (v *PropertyValidator) fixPropertyValue(prop types.PropertyItem, templateProp TemuPropertyOption) *types.PropertyItem {
	switch templateProp.PropertyValueType {
	case 1: // 选择类型
		// 尝试通过值匹配找到正确的VID
		for _, value := range templateProp.Values {
			if strings.EqualFold(value.Value, prop.Value) {
				v.logger.Infof("✅ 通过值匹配修复属性 %s: %s -> VID=%d",
					templateProp.Name, prop.Value, value.VID)
				return &types.PropertyItem{
					RefPid:           templateProp.RefPID,
					Pid:              templateProp.PID,
					TemplatePid:      templateProp.TemplatePID,
					TemplateModuleID: templateProp.TemplateModuleID,
					Value:            value.Value,
					Vid:              value.VID,
					ValueUnit:        prop.ValueUnit,
				}
			}
		}

		// 如果找不到匹配，尝试找一个合适的默认值
		// 优先选择英文中性选项，避免中文字符
		neutralKeywords := []string{
			"N/A", "None", "Other", "Not Applicable", "No", "Without",
			"不适用", "无", "其他", "无需", "不含",
		}
		for _, keyword := range neutralKeywords {
			for _, value := range templateProp.Values {
				if strings.Contains(value.Value, keyword) {
					// 检查是否包含中文字符
					hasChinese := false
					for _, r := range value.Value {
						if r >= 0x4e00 && r <= 0x9fff {
							hasChinese = true
							break
						}
					}

					// 优先使用不包含中文的选项
					if !hasChinese {
						v.logger.Infof("✅ 使用英文中性默认值修复属性 %s: %s (VID=%d)",
							templateProp.Name, value.Value, value.VID)
						return &types.PropertyItem{
							RefPid:           templateProp.RefPID,
							Pid:              templateProp.PID,
							TemplatePid:      templateProp.TemplatePID,
							TemplateModuleID: templateProp.TemplateModuleID,
							Value:            value.Value,
							Vid:              value.VID,
							ValueUnit:        prop.ValueUnit,
						}
					}
				}
			}
		}

		// 如果没有找到英文中性选项，再尝试中文选项（作为最后的备选）
		for _, keyword := range neutralKeywords {
			for _, value := range templateProp.Values {
				if strings.Contains(value.Value, keyword) {
					v.logger.Warnf("⚠️ 只找到中文中性默认值修复属性 %s: %s (VID=%d)",
						templateProp.Name, value.Value, value.VID)
					return &types.PropertyItem{
						RefPid:           templateProp.RefPID,
						Pid:              templateProp.PID,
						TemplatePid:      templateProp.TemplatePID,
						TemplateModuleID: templateProp.TemplateModuleID,
						Value:            value.Value,
						Vid:              value.VID,
						ValueUnit:        prop.ValueUnit,
					}
				}
			}
		}

		// 如果没有中性选项，使用第一个可选值作为默认值
		if len(templateProp.Values) > 0 {
			v.logger.Warnf("⚠️ 使用第一个可选值修复属性 %s: %s (VID=%d)",
				templateProp.Name, templateProp.Values[0].Value, templateProp.Values[0].VID)
			return &types.PropertyItem{
				RefPid:           templateProp.RefPID,
				Pid:              templateProp.PID,
				TemplatePid:      templateProp.TemplatePID,
				TemplateModuleID: templateProp.TemplateModuleID,
				Value:            templateProp.Values[0].Value,
				Vid:              templateProp.Values[0].VID,
				ValueUnit:        prop.ValueUnit,
			}
		}
	case 2, 3: // 数字类型或文本类型
		if prop.Value != "" {
			// 确保使用正确的template_pid和ref_pid
			prop.RefPid = templateProp.RefPID
			prop.TemplatePid = templateProp.TemplatePID
			prop.TemplateModuleID = templateProp.TemplateModuleID
			return &prop
		}
	}
	return nil
}

// createDefaultProperty 为必填属性创建默认值
func (v *PropertyValidator) createDefaultProperty(templateProp TemuPropertyOption) *types.PropertyItem {
	prop := &types.PropertyItem{
		RefPid:           templateProp.RefPID,
		Pid:              templateProp.PID,
		TemplatePid:      templateProp.TemplatePID,
		TemplateModuleID: templateProp.TemplateModuleID,
	}

	switch templateProp.PropertyValueType {
	case 1: // 选择类型
		if len(templateProp.Values) > 0 {
			prop.Value = templateProp.Values[0].Value
			prop.Vid = templateProp.Values[0].VID
		}
	case 2: // 数字类型
		if templateProp.MinValue != "" {
			prop.Value = templateProp.MinValue
		} else {
			prop.Value = "1"
		}
	case 3: // 文本类型
		prop.Value = "未指定"
	}

	// 设置单位
	if len(templateProp.ValueUnit) > 0 {
		prop.ValueUnit = templateProp.ValueUnit[0]
	}

	return prop
}
