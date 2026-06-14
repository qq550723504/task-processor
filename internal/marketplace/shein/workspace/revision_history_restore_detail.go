package workspace

type HistoryRestoreTimeline struct {
	Headline     string
	RelationText string
}

type HistoryRestoreRecordInput struct {
	RevisionID             string
	Platform               string
	ActionType             string
	RestoredFromRevisionID string
	Timeline               *HistoryRestoreTimeline
}

type HistoryRestoreCompareInput struct {
	CompareTo         string
	CompareRevisionID string
	RelationLabel     string
	ChangeCount       int
}

type HistoryRestoreStateInput struct {
	HasCurrentPackage     bool
	CategoryResolved      bool
	AttributeResolved     bool
	SaleAttributeResolved bool
	ManualReviewNotes     []string
}

type HistoryRestoreContext struct {
	SourceRevisionID string `json:"source_revision_id,omitempty"`
	SourceActionType string `json:"source_action_type,omitempty"`
	SourceHeadline   string `json:"source_headline,omitempty"`
	TargetRevisionID string `json:"target_revision_id,omitempty"`
	TargetLabel      string `json:"target_label,omitempty"`
	CompareMode      string `json:"compare_mode,omitempty"`
	ExecutionMode    string `json:"execution_mode,omitempty"`
	RestoreReason    string `json:"restore_reason,omitempty"`
	RestorePlatform  string `json:"restore_platform,omitempty"`
}

type HistoryRestoreSafety struct {
	CanRestore      bool     `json:"can_restore"`
	RestoreWarnings []string `json:"restore_warnings,omitempty"`
}

type HistoryRestoreOverview struct {
	Status        string   `json:"status,omitempty"`
	Headline      string   `json:"headline,omitempty"`
	Subheadline   string   `json:"subheadline,omitempty"`
	PrimaryAction string   `json:"primary_action,omitempty"`
	NextActions   []string `json:"next_actions,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
}

type HistoryRestoreMessages struct {
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	ConfirmLabel     string   `json:"confirm_label,omitempty"`
	CancelLabel      string   `json:"cancel_label,omitempty"`
	WarningTitle     string   `json:"warning_title,omitempty"`
	WarningSummaries []string `json:"warning_summaries,omitempty"`
}

func BuildHistoryRestoreContext(record *HistoryRestoreRecordInput, executionMode, platform, reason string, compare *HistoryRestoreCompareInput) *HistoryRestoreContext {
	if record == nil && compare == nil && platform == "" && reason == "" && executionMode == "" {
		return nil
	}

	context := &HistoryRestoreContext{
		ExecutionMode:   firstNonEmpty(executionMode, "restore_from_revision_id"),
		RestorePlatform: platform,
		RestoreReason:   reason,
	}
	if record != nil {
		context.SourceRevisionID = record.RevisionID
		context.SourceActionType = record.ActionType
		if record.Timeline != nil {
			context.SourceHeadline = record.Timeline.Headline
		}
	}
	if compare != nil {
		context.CompareMode = compare.CompareTo
		context.TargetRevisionID = compare.CompareRevisionID
		context.TargetLabel = compare.RelationLabel
	}
	if context.TargetRevisionID == "" {
		context.TargetRevisionID = "current"
	}
	if context.TargetLabel == "" {
		context.TargetLabel = "当前版本"
	}
	return context
}

func BuildHistoryRestoreSafety(state *HistoryRestoreStateInput, record *HistoryRestoreRecordInput, draft *EditorRevisionSkeleton, compare *HistoryRestoreCompareInput) *HistoryRestoreSafety {
	safety := &HistoryRestoreSafety{}
	if record == nil {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前历史记录缺少恢复上下文，暂时不能直接回滚")
		return safety
	}
	if record.Platform != "shein" {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前历史记录不是 SHEIN 资料包，暂时不支持直接恢复")
		return safety
	}
	if state == nil || !state.HasCurrentPackage {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前任务没有可用的 SHEIN 资料包，恢复前需要先生成 SHEIN 结果")
		return safety
	}
	if draft == nil || draft.Shein == nil {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前历史记录缺少 restore_draft，暂时不能直接恢复")
		return safety
	}

	safety.CanRestore = true
	if compare != nil && compare.CompareTo == "current" && compare.ChangeCount == 0 {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "这条历史与当前版本没有差异，执行恢复不会带来实际变化")
	}
	if !state.CategoryResolved {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本的类目骨架仍未完全解析，恢复后建议重新确认 category_id 和 product_type_id")
	}
	if !state.AttributeResolved {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本的普通属性仍有未解析项，恢复后建议再次检查 attribute_id 映射")
	}
	if !state.SaleAttributeResolved {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本的销售属性还未完全稳定，恢复后建议再次确认主副规格映射")
	}
	if len(state.ManualReviewNotes) > 0 {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "当前版本仍有人工备注待处理，恢复后建议再核对这些备注是否仍然适用")
	}
	if record.ActionType == "restore" && record.RestoredFromRevisionID != "" {
		safety.RestoreWarnings = append(safety.RestoreWarnings, "这条历史本身来自一次回滚操作，恢复后请留意是否会重复覆盖较新的手工修改")
	}
	safety.RestoreWarnings = uniqueStrings(safety.RestoreWarnings)
	return safety
}

func BuildHistoryRestoreOverview(record *HistoryRestoreRecordInput, safety *HistoryRestoreSafety, compare *HistoryRestoreCompareInput) *HistoryRestoreOverview {
	if record == nil && safety == nil && compare == nil {
		return nil
	}

	overview := &HistoryRestoreOverview{
		Status:        "ready",
		Headline:      "恢复这条历史版本",
		PrimaryAction: "恢复历史版本",
		NextActions:   BuildHistoryRestoreNextActions(record, safety, compare),
	}
	if record != nil && record.Timeline != nil && record.Timeline.Headline != "" {
		overview.Headline = record.Timeline.Headline
	}
	if safety != nil {
		switch {
		case !safety.CanRestore:
			overview.Status = "blocked"
			overview.PrimaryAction = "暂不建议恢复"
			overview.Subheadline = "当前上下文还不满足直接回滚条件，建议先处理阻塞项"
		case len(safety.RestoreWarnings) > 0:
			overview.Status = "ready_with_warnings"
			overview.Subheadline = "可以恢复，但建议先确认潜在影响"
		default:
			overview.Subheadline = "当前可以直接恢复到这条历史版本"
		}
		overview.Highlights = append(overview.Highlights, safety.RestoreWarnings...)
	}
	if compare != nil {
		if compare.RelationLabel != "" {
			overview.Highlights = append(overview.Highlights, "比较目标："+compare.RelationLabel)
		}
		switch {
		case compare.ChangeCount > 0:
			overview.Highlights = append(overview.Highlights, "恢复后预计会影响 "+formatInt(compare.ChangeCount)+" 个字段")
		case compare.CompareTo != "":
			overview.Highlights = append(overview.Highlights, "恢复后与"+firstNonEmpty(compare.RelationLabel, "比较目标")+"没有字段差异")
		}
	}
	if record != nil && record.Timeline != nil && record.Timeline.RelationText != "" {
		overview.Highlights = append(overview.Highlights, record.Timeline.RelationText)
	}
	overview.NextActions = uniqueStrings(overview.NextActions)
	overview.Highlights = uniqueStrings(overview.Highlights)
	return overview
}

func BuildHistoryRestoreMessages(context *HistoryRestoreContext, safety *HistoryRestoreSafety, overview *HistoryRestoreOverview) *HistoryRestoreMessages {
	if context == nil && safety == nil && overview == nil {
		return nil
	}
	msg := &HistoryRestoreMessages{
		Title:        "确认恢复这条历史版本",
		CancelLabel:  "取消",
		ConfirmLabel: "确认恢复",
	}
	if overview != nil {
		if overview.Headline != "" {
			msg.Title = overview.Headline
		}
		if overview.PrimaryAction != "" {
			msg.ConfirmLabel = overview.PrimaryAction
		}
	}
	if context != nil {
		source := firstNonEmpty(context.SourceRevisionID, "当前记录")
		target := firstNonEmpty(context.TargetLabel, "当前版本")
		msg.Description = "将从 " + source + " 恢复，并与" + target + "进行对齐。"
	}
	if safety != nil && !safety.CanRestore {
		msg.WarningTitle = "当前不建议直接恢复"
		msg.ConfirmLabel = "暂不恢复"
	} else if safety != nil && len(safety.RestoreWarnings) > 0 {
		msg.WarningTitle = "恢复前建议先确认以下事项"
	}
	if safety != nil {
		msg.WarningSummaries = append([]string(nil), safety.RestoreWarnings...)
	}
	msg.WarningSummaries = uniqueStrings(msg.WarningSummaries)
	return msg
}

func BuildHistoryRestoreNextActions(record *HistoryRestoreRecordInput, safety *HistoryRestoreSafety, compare *HistoryRestoreCompareInput) []string {
	actions := make([]string, 0, 4)
	if safety == nil {
		return actions
	}
	if !safety.CanRestore {
		return append(actions, "先生成或恢复当前 SHEIN 资料包")
	}
	if hasHistoryRestoreWarning(safety, "类目骨架") {
		actions = append(actions, "先确认类目")
	}
	if hasHistoryRestoreWarning(safety, "普通属性") || hasHistoryRestoreWarning(safety, "attribute_id") {
		actions = append(actions, "先确认属性")
	}
	if hasHistoryRestoreWarning(safety, "销售属性") || hasHistoryRestoreWarning(safety, "主副规格") {
		actions = append(actions, "先确认规格")
	}
	if hasHistoryRestoreWarning(safety, "人工备注") || hasHistoryRestoreWarning(safety, "备注") {
		actions = append(actions, "先处理人工备注")
	}
	if compare != nil {
		switch {
		case compare.ChangeCount == 0 && compare.CompareTo != "":
			actions = append(actions, "当前无需恢复")
		case compare.ChangeCount > 0:
			actions = append(actions, "确认后执行恢复")
		}
	}
	if len(actions) == 0 {
		if record != nil && record.ActionType == "restore" {
			actions = append(actions, "检查回滚来源")
		}
		actions = append(actions, "直接恢复历史版本")
	}
	return uniqueStrings(actions)
}

func hasHistoryRestoreWarning(safety *HistoryRestoreSafety, pattern string) bool {
	if safety == nil || pattern == "" {
		return false
	}
	target := normalizeText(pattern)
	for _, warning := range safety.RestoreWarnings {
		if stringsContainsNormalized(warning, target) {
			return true
		}
	}
	return false
}

func stringsContainsNormalized(value, pattern string) bool {
	return value != "" && pattern != "" && containsNormalized(normalizeText(value), pattern)
}

func containsNormalized(value, pattern string) bool {
	if value == "" || pattern == "" {
		return false
	}
	return len(value) >= len(pattern) && indexNormalized(value, pattern) >= 0
}

func indexNormalized(value, pattern string) int {
	for i := 0; i+len(pattern) <= len(value); i++ {
		if value[i:i+len(pattern)] == pattern {
			return i
		}
	}
	return -1
}
