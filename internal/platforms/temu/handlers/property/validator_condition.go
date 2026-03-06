package property

import (
	"task-processor/internal/platforms/temu/api/models"
	"task-processor/internal/platforms/temu/types"
)

// filterByPropertyRelations 根据属性关联关系过滤属性（基于ShowCondition）
func (v *PropertyValidator) filterByPropertyRelations(properties []models.PropertyItem, data types.PropertyMappingData) []models.PropertyItem {
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
	pidToTemplate := make(map[int]types.TemplateRespGoodsProperty)
	for _, templateProp := range data.TemuProperties {
		pidToTemplate[templateProp.PID] = templateProp
	}

	// 过滤属性
	var filteredProperties []models.PropertyItem

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
func (v *PropertyValidator) checkShowCondition(templateProp types.TemplateRespGoodsProperty, refPidToSelectedVids map[int]map[int]bool) bool {
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
