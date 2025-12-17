package handlers

import "task-processor/internal/platforms/temu/types"

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
