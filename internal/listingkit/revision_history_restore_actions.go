package listingkit

func buildRevisionHistoryRestoreNextActions(record *ListingKitRevisionRecord, safety *RevisionHistoryRestoreSafety, comparePreview *RevisionHistoryComparePreview) []string {
	actions := make([]string, 0, 4)

	if safety == nil {
		return actions
	}

	if !safety.CanRestore {
		actions = append(actions, "先生成或恢复当前 SHEIN 资料包")
		return actions
	}

	if hasRevisionHistoryRestoreWarning(safety, "类目骨架") {
		actions = append(actions, "先确认类目")
	}
	if hasRevisionHistoryRestoreWarning(safety, "普通属性") || hasRevisionHistoryRestoreWarning(safety, "attribute_id") {
		actions = append(actions, "先确认属性")
	}
	if hasRevisionHistoryRestoreWarning(safety, "销售属性") || hasRevisionHistoryRestoreWarning(safety, "主副规格") {
		actions = append(actions, "先确认规格")
	}
	if hasRevisionHistoryRestoreWarning(safety, "人工备注") || hasRevisionHistoryRestoreWarning(safety, "备注") {
		actions = append(actions, "先处理人工备注")
	}

	if comparePreview != nil && comparePreview.DiffPreview != nil {
		switch {
		case comparePreview.DiffPreview.ChangeCount == 0:
			actions = append(actions, "当前无需恢复")
		case comparePreview.DiffPreview.ChangeCount > 0:
			actions = append(actions, "确认后执行恢复")
		}
	}

	if len(actions) == 0 {
		if record != nil && record.ActionType == RevisionActionTypeRestore {
			actions = append(actions, "检查回滚来源")
		}
		actions = append(actions, "直接恢复历史版本")
	}

	return actions
}

func hasRevisionHistoryRestoreWarning(safety *RevisionHistoryRestoreSafety, pattern string) bool {
	if safety == nil || pattern == "" {
		return false
	}
	for _, warning := range safety.RestoreWarnings {
		if containsText(warning, pattern) {
			return true
		}
	}
	return false
}
