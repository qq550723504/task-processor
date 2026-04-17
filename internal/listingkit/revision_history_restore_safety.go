package listingkit

type RevisionHistoryRestoreSafety struct {
	CanRestore      bool     `json:"can_restore"`
	RestoreWarnings []string `json:"restore_warnings,omitempty"`
}

func buildRevisionHistoryRestoreSafety(result *ListingKitResult, record *ListingKitRevisionRecord, restoreDraft *SheinEditorRevisionSkeleton, comparePreview *RevisionHistoryComparePreview) *RevisionHistoryRestoreSafety {
	safety := &RevisionHistoryRestoreSafety{}
	if record == nil {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前历史记录缺少恢复上下文，暂时不能直接回滚")
		return safety
	}

	if record.Platform != "shein" {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前历史记录不是 SHEIN 资料包，暂时不支持直接恢复")
		return safety
	}

	if result == nil || result.Shein == nil {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前任务没有可用的 SHEIN 资料包，恢复前需要先生成 SHEIN 结果")
		return safety
	}

	if restoreDraft == nil || restoreDraft.Shein == nil {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前历史记录缺少 restore_draft，暂时不能直接恢复")
		return safety
	}

	safety.CanRestore = true

	if comparePreview != nil && comparePreview.CompareTo == "current" && comparePreview.DiffPreview != nil && comparePreview.DiffPreview.ChangeCount == 0 {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "这条历史与当前版本没有差异，执行恢复不会带来实际变化")
	}

	if !isSheinCategoryResolved(result.Shein) {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本的类目骨架仍未完全解析，恢复后建议重新确认 category_id 和 product_type_id")
	}
	if !isSheinAttributeResolved(result.Shein) {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本的普通属性仍有未解析项，恢复后建议再次检查 attribute_id 映射")
	}
	if !isSheinSaleAttributeResolved(result.Shein) {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本的销售属性还未完全稳定，恢复后建议再次确认主副规格映射")
	}

	manualNotes := filterManualSheinReviewNotes(result.Shein.ReviewNotes)
	if len(manualNotes) > 0 {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本仍有人工备注待处理，恢复后建议再核对这些备注是否仍然适用")
	}

	if record.ActionType == RevisionActionTypeRestore && record.RestoredFromRevisionID != "" {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "这条历史本身来自一次回滚操作，恢复后请留意是否会重复覆盖较新的手工修改")
	}

	safety.RestoreWarnings = uniqueStrings(safety.RestoreWarnings)
	return safety
}
