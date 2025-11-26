package handlers

import (
	"strings"

	"task-processor/platforms/temu/types"

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

// deduplicateProperties 去重属性，确保单选属性的RefPID只出现一次，优先保留必填属性
func (v *PropertyValidator) deduplicateProperties(properties []types.PropertyItem, templateProps []TemuPropertyOption) []types.PropertyItem {
	v.logger.Infof("🔄【去重开始】处理 %d 个属性，模板属性 %d 个", len(properties), len(templateProps))

	// 创建模板属性映射
	templateMap := make(map[int]TemuPropertyOption)
	for _, prop := range templateProps {
		templateMap[prop.PID] = prop
	}

	// 按RefPID分组属性，并按必填优先级排序
	refPidGroups := make(map[int][]types.PropertyItem)
	for _, prop := range properties {
		refPidGroups[prop.RefPid] = append(refPidGroups[prop.RefPid], prop)
	}

	// 对每个RefPID组内的属性按必填优先级排序
	for refPid, group := range refPidGroups {
		// 将必填属性排在前面
		sortedGroup := make([]types.PropertyItem, 0, len(group))
		var optionalProps []types.PropertyItem

		for _, prop := range group {
			if templateProp, exists := templateMap[prop.Pid]; exists && templateProp.Required {
				sortedGroup = append(sortedGroup, prop)
			} else {
				optionalProps = append(optionalProps, prop)
			}
		}
		sortedGroup = append(sortedGroup, optionalProps...)
		refPidGroups[refPid] = sortedGroup
	}

	// 跟踪每个RefPID的出现次数
	refPidCounts := make(map[int]int)
	var deduplicatedProperties []types.PropertyItem

	// 按RefPID处理属性
	for refPid, group := range refPidGroups {
		for _, prop := range group {
			templateProp, exists := templateMap[prop.Pid]
			if !exists {
				v.logger.Warnf("❌【去重】属性PID %d 不在模板中，跳过", prop.Pid)
				continue
			}

			currentCount := refPidCounts[refPid]
			maxAllowed := 1 // 默认每个RefPID只允许1个

			if templateProp.PropertyValueType == 1 && templateProp.ChooseMaxNum > 0 {
				maxAllowed = templateProp.ChooseMaxNum
			}

			if currentCount >= maxAllowed {
				if templateProp.Required {
					v.logger.Warnf("⚠️【去重】必填属性 %s (PID=%d, RefPID=%d) 因数量限制被跳过，当前数量=%d，最大允许=%d",
						templateProp.Name, prop.Pid, refPid, currentCount, maxAllowed)
				}
				continue
			}

			deduplicatedProperties = append(deduplicatedProperties, prop)
			refPidCounts[refPid]++
		}
	}

	return deduplicatedProperties
}

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

// filterByPropertyRelations 根据属性关联关系过滤属性（基于ShowCondition）
func (v *PropertyValidator) filterByPropertyRelations(properties []types.PropertyItem, data PropertyMappingData) []types.PropertyItem {
	v.logger.Info("开始应用属性关联过滤规则（基于ShowCondition）")

	// 创建RefPID到已选择VID的映射
	refPidToSelectedVids := make(map[int]map[int]bool)

	for _, prop := range properties {
		if _, exists := refPidToSelectedVids[prop.RefPid]; !exists {
			refPidToSelectedVids[prop.RefPid] = make(map[int]bool)
		}
		refPidToSelectedVids[prop.RefPid][prop.Vid] = true
	}

	v.logger.Infof("已选择的属性: %d个RefPID", len(refPidToSelectedVids))
	for refPid, vids := range refPidToSelectedVids {
		v.logger.Debugf("  RefPID=%d, VIDs=%v", refPid, getKeys(vids))
	}

	// 创建PID到模板属性的映射
	pidToTemplate := make(map[int]TemuPropertyOption)
	for _, templateProp := range data.TemuProperties {
		pidToTemplate[templateProp.PID] = templateProp
	}

	// 过滤属性
	var filteredProperties []types.PropertyItem

	for _, prop := range properties {
		templateProp, exists := pidToTemplate[prop.Pid]
		if !exists {
			v.logger.Warnf("属性PID=%d不在模板中，保留", prop.Pid)
			filteredProperties = append(filteredProperties, prop)
			continue
		}

		// 检查ShowCondition
		shouldKeep := v.checkShowCondition(templateProp, refPidToSelectedVids)

		if shouldKeep {
			filteredProperties = append(filteredProperties, prop)
		} else {
			v.logger.Infof("根据ShowCondition过滤掉属性: %s (RefPID=%d, PID=%d)",
				templateProp.Name, templateProp.RefPID, templateProp.PID)
		}
	}

	v.logger.Infof("属性关联过滤完成: 原始=%d, 过滤后=%d", len(properties), len(filteredProperties))
	return filteredProperties
}

// checkShowCondition 检查属性的显示条件是否满足
func (v *PropertyValidator) checkShowCondition(templateProp TemuPropertyOption, refPidToSelectedVids map[int]map[int]bool) bool {
	// 如果没有ShowCondition，说明无条件显示
	if len(templateProp.ShowCondition) == 0 {
		return true
	}

	// 检查所有ShowCondition，只要有一个满足就显示
	for _, condition := range templateProp.ShowCondition {
		selectedVids, hasParent := refPidToSelectedVids[condition.ParentRefPID]
		if !hasParent {
			// 父属性未选择，条件不满足
			v.logger.Debugf("属性%s的ShowCondition不满足: 父属性RefPID=%d未选择",
				templateProp.Name, condition.ParentRefPID)
			continue
		}

		// 检查是否选择了ParentVIDs中的任意一个值
		conditionMet := false
		for _, requiredVid := range condition.ParentVIDs {
			if selectedVids[requiredVid] {
				conditionMet = true
				v.logger.Debugf("属性%s的ShowCondition满足: 父属性RefPID=%d选择了VID=%d",
					templateProp.Name, condition.ParentRefPID, requiredVid)
				break
			}
		}

		if conditionMet {
			return true
		}
	}

	// 所有ShowCondition都不满足
	v.logger.Infof("属性%s的所有ShowCondition都不满足，过滤该属性", templateProp.Name)
	return false
}

// getKeys 获取map的所有key
func getKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
