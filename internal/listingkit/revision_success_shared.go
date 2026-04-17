package listingkit

type revisionSuccessMode string

const (
	revisionSuccessModeApply   revisionSuccessMode = "apply"
	revisionSuccessModeRestore revisionSuccessMode = "restore"
)

type revisionSuccessFollowUpData struct {
	Checklist         *RevisionFollowUpChecklist
	Overview          *RevisionFollowUpOverview
	SuggestedRevision *SheinEditorRevisionSkeleton
}

func buildRevisionSuccessNextActions(result *ListingKitResult) []string {
	if result == nil || result.Shein == nil {
		return nil
	}

	actions := make([]string, 0, 4)
	if !isSheinCategoryResolved(result.Shein) {
		actions = append(actions, "复查类目")
	}
	if !isSheinAttributeResolved(result.Shein) {
		actions = append(actions, "复查属性")
	}
	if !isSheinSaleAttributeResolved(result.Shein) {
		actions = append(actions, "复查规格")
	}
	if len(filterManualSheinReviewNotes(result.Shein.ReviewNotes)) > 0 {
		actions = append(actions, "处理人工备注")
	}
	if len(actions) == 0 {
		actions = append(actions, "继续提交流程")
	}
	return uniqueStrings(actions)
}

func buildRevisionSuccessStatusSummary(result *ListingKitResult) *RevisionStatusSummary {
	if result == nil || result.Shein == nil {
		return nil
	}

	readiness := buildSheinSubmitReadiness(result.Shein)
	overview := buildSheinStatusOverview(result.Shein, readiness)
	if overview == nil {
		return nil
	}

	return &RevisionStatusSummary{
		Status:        overview.Status,
		Headline:      overview.Headline,
		Subheadline:   overview.Subheadline,
		NeedsReview:   overview.NeedsReview,
		BlockingCount: overview.BlockingCount,
		WarningCount:  overview.WarningCount,
		Highlights:    append([]string(nil), overview.Highlights...),
	}
}

func buildRevisionSuccessMessages(mode revisionSuccessMode, headline string, changeCount int, sourceRevisionID string, summary *RevisionStatusSummary) *RevisionResultMessages {
	messages := &RevisionResultMessages{
		Title:        firstNonEmpty(headline, defaultRevisionSuccessTitle(mode)),
		SuccessLabel: defaultRevisionSuccessLabel(mode),
	}

	switch mode {
	case revisionSuccessModeRestore:
		if sourceRevisionID != "" {
			messages.Description = "已恢复到历史版本 " + sourceRevisionID
			if changeCount > 0 {
				messages.Description += "，共覆盖 " + formatInt(changeCount) + " 个字段。"
			} else {
				messages.Description += "。"
			}
		}
	default:
		if changeCount > 0 {
			messages.Description = "本次已保存 " + formatInt(changeCount) + " 个字段的更新。"
		} else {
			messages.Description = "资料已保存。"
		}
	}

	if summary != nil && summary.NeedsReview {
		messages.WarningTitle = defaultRevisionSuccessWarningTitle(mode)
		messages.WarningSummaries = append(messages.WarningSummaries, summary.Subheadline)
		messages.WarningSummaries = append(messages.WarningSummaries, summary.Highlights...)
	}
	messages.WarningSummaries = uniqueStrings(filterNonEmptyStrings(messages.WarningSummaries))
	return messages
}

func buildRevisionSuccessRecommendedView(mode revisionSuccessMode, result *ListingKitResult, summary *RevisionStatusSummary) *RevisionRecommendedView {
	if result == nil || result.Shein == nil {
		return nil
	}

	view := &RevisionRecommendedView{}
	switch {
	case summary != nil && summary.Status == "blocked":
		view.View = "editor"
		view.Reason = firstNonEmpty(summary.Subheadline, defaultRevisionSuccessEditorReason(mode))
	case summary != nil && summary.NeedsReview:
		view.View = "inspection"
		view.Reason = firstNonEmpty(summary.Subheadline, defaultRevisionSuccessInspectionReason(mode))
	default:
		view.View = "submit"
		view.Reason = defaultRevisionSuccessSubmitReason(mode)
	}
	return view
}

func buildRevisionSuccessFollowUpChecklist(result *ListingKitResult) *RevisionFollowUpChecklist {
	if result == nil || result.Shein == nil {
		return nil
	}

	checklist := buildSheinSubmitChecklist(buildSheinSubmitReadiness(result.Shein))
	if checklist == nil {
		return nil
	}

	out := &RevisionFollowUpChecklist{
		Required:    append([]SheinChecklistGroupItem(nil), checklist.Required...),
		Recommended: append([]SheinChecklistGroupItem(nil), checklist.Recommended...),
	}
	if len(out.Required) == 0 && len(out.Recommended) == 0 {
		return nil
	}
	return out
}

func buildRevisionSuccessSuggestedFollowUpRevision(mode revisionSuccessMode, result *ListingKitResult) *SheinEditorRevisionSkeleton {
	if result == nil || result.Shein == nil {
		return nil
	}
	skeleton := buildSheinMinimalRevisionSkeleton(result.Shein)
	if skeleton == nil {
		return nil
	}
	skeleton.Actor = "desktop-client"
	switch mode {
	case revisionSuccessModeRestore:
		skeleton.Reason = "follow-up after restore"
	default:
		skeleton.Reason = "follow-up after apply"
	}
	return skeleton
}

func buildRevisionSuccessFollowUpData(
	mode revisionSuccessMode,
	result *ListingKitResult,
	summary *RevisionStatusSummary,
	messages *RevisionResultMessages,
	nextActions []string,
) *revisionSuccessFollowUpData {
	checklist := buildRevisionSuccessFollowUpChecklist(result)
	overview := buildRevisionSuccessFollowUpOverview(mode, summary, messages, checklist, nextActions)
	suggested := buildRevisionSuccessSuggestedFollowUpRevision(mode, result)
	if checklist == nil && overview == nil && suggested == nil {
		return nil
	}
	return &revisionSuccessFollowUpData{
		Checklist:         checklist,
		Overview:          overview,
		SuggestedRevision: suggested,
	}
}

func buildRevisionSuccessFollowUpOverview(mode revisionSuccessMode, summary *RevisionStatusSummary, messages *RevisionResultMessages, checklist *RevisionFollowUpChecklist, nextActions []string) *RevisionFollowUpOverview {
	overview := &RevisionFollowUpOverview{
		Status:      firstNonEmpty(summaryStatusValue(summary), "ready"),
		Headline:    buildRevisionSuccessFollowUpHeadline(mode, summary),
		Subheadline: firstNonEmpty(summarySubheadlineValue(summary), messageDescriptionValue(messages)),
		NextActions: append([]string(nil), nextActions...),
	}
	if checklist != nil {
		overview.RequiredCount = len(checklist.Required)
		overview.RecommendedCount = len(checklist.Recommended)
	}
	if overview.Headline == "" && overview.Subheadline == "" && len(overview.NextActions) == 0 &&
		overview.RequiredCount == 0 && overview.RecommendedCount == 0 {
		return nil
	}
	return overview
}

func buildRevisionSuccessFollowUpHeadline(mode revisionSuccessMode, summary *RevisionStatusSummary) string {
	switch {
	case summary == nil:
		return ""
	case summary.Status == "blocked":
		if mode == revisionSuccessModeRestore {
			return "恢复后仍需继续补齐资料"
		}
		return "保存后仍需继续补齐资料"
	case summary.NeedsReview:
		if mode == revisionSuccessModeRestore {
			return "恢复后建议先继续检查"
		}
		return "保存后建议先继续检查"
	default:
		if mode == revisionSuccessModeRestore {
			return "恢复后可以继续提交流程"
		}
		return "保存后可以继续提交流程"
	}
}

func defaultRevisionSuccessTitle(mode revisionSuccessMode) string {
	if mode == revisionSuccessModeRestore {
		return "历史版本已恢复"
	}
	return "资料已更新"
}

func defaultRevisionSuccessLabel(mode revisionSuccessMode) string {
	if mode == revisionSuccessModeRestore {
		return "恢复成功"
	}
	return "保存成功"
}

func defaultRevisionSuccessWarningTitle(mode revisionSuccessMode) string {
	if mode == revisionSuccessModeRestore {
		return "恢复后仍建议继续确认"
	}
	return "保存后仍建议继续确认"
}

func defaultRevisionSuccessEditorReason(mode revisionSuccessMode) string {
	if mode == revisionSuccessModeRestore {
		return "恢复后仍有关键字段待补齐，建议继续编辑。"
	}
	return "保存后仍有关键字段待补齐，建议继续编辑。"
}

func defaultRevisionSuccessInspectionReason(mode revisionSuccessMode) string {
	if mode == revisionSuccessModeRestore {
		return "恢复后仍有待确认项，建议先检查。"
	}
	return "保存后仍有待确认项，建议先检查。"
}

func defaultRevisionSuccessSubmitReason(mode revisionSuccessMode) string {
	if mode == revisionSuccessModeRestore {
		return "恢复后可以直接继续提交流程。"
	}
	return "保存后可以直接继续提交流程。"
}
