package shein

import sheinpub "task-processor/internal/publishing/shein"

const (
	SuccessModeApply    = "apply"
	SuccessModeRestore  = "restore"
	SceneApplySuccess   = "apply_success"
	SceneRestoreSuccess = "restore_success"
)

func BuildSuccessNextActions(pkg *sheinpub.Package) []string {
	if pkg == nil {
		return nil
	}
	actions := make([]string, 0, 4)
	if !IsCategoryResolved(pkg) {
		actions = append(actions, "复查类目")
	}
	if !IsAttributeResolved(pkg) {
		actions = append(actions, "复查属性")
	}
	if !IsSaleAttributeResolved(pkg) {
		actions = append(actions, "复查规格")
	}
	if len(FilterManualReviewNotes(pkg.ReviewNotes)) > 0 {
		actions = append(actions, "处理人工备注")
	}
	if len(actions) == 0 {
		actions = append(actions, "继续提交流程")
	}
	return uniqueStrings(actions)
}

func BuildSuccessStatusSummary[Reason any, Hint any](pkg *sheinpub.Package, readiness *SubmitReadiness[Reason, Hint]) *SuccessStatusSummary {
	if pkg == nil {
		return nil
	}
	overview := BuildStatusOverview(pkg.Inspection, toSubmitStateInput(readiness))
	if overview == nil {
		return nil
	}
	return &SuccessStatusSummary{
		Status:        overview.Status,
		Headline:      overview.Headline,
		Subheadline:   overview.Subheadline,
		NeedsReview:   overview.NeedsReview,
		BlockingCount: overview.BlockingCount,
		WarningCount:  overview.WarningCount,
		Highlights:    append([]string(nil), overview.Highlights...),
	}
}

func BuildSuccessMessages(mode, headline string, changeCount int, sourceRevisionID string, summary *SuccessStatusSummary) *SuccessMessages {
	msg := &SuccessMessages{
		Title:        firstNonEmpty(headline, defaultSuccessTitle(mode)),
		SuccessLabel: defaultSuccessLabel(mode),
	}
	switch mode {
	case SuccessModeRestore:
		if sourceRevisionID != "" {
			msg.Description = "已恢复到历史版本 " + sourceRevisionID
			if changeCount > 0 {
				msg.Description += "，共覆盖 " + formatInt(changeCount) + " 个字段。"
			} else {
				msg.Description += "。"
			}
		}
	default:
		if changeCount > 0 {
			msg.Description = "本次已保存 " + formatInt(changeCount) + " 个字段的更新。"
		} else {
			msg.Description = "资料已保存。"
		}
	}
	if summary != nil && summary.NeedsReview {
		msg.WarningTitle = defaultSuccessWarningTitle(mode)
		msg.WarningSummaries = append(msg.WarningSummaries, summary.Subheadline)
		msg.WarningSummaries = append(msg.WarningSummaries, summary.Highlights...)
	}
	msg.WarningSummaries = uniqueStrings(msg.WarningSummaries)
	return msg
}

func BuildSuccessRecommendedView(mode string, summary *SuccessStatusSummary) *SuccessRecommendedView {
	if summary == nil {
		return nil
	}
	view := &SuccessRecommendedView{}
	switch {
	case summary.Status == "blocked":
		view.View = "editor"
		view.Reason = firstNonEmpty(summary.Subheadline, defaultSuccessEditorReason(mode))
	case summary.NeedsReview:
		view.View = "inspection"
		view.Reason = firstNonEmpty(summary.Subheadline, defaultSuccessInspectionReason(mode))
	default:
		view.View = "submit"
		view.Reason = defaultSuccessSubmitReason(mode)
	}
	return view
}

func BuildSuccessFollowUpChecklist[Reason any, Hint any](checklist *SubmitChecklist[Reason, Hint]) *SuccessFollowUpChecklist[ChecklistGroupItem[Reason, Hint]] {
	if checklist == nil {
		return nil
	}
	out := &SuccessFollowUpChecklist[ChecklistGroupItem[Reason, Hint]]{
		Required:    append([]ChecklistGroupItem[Reason, Hint](nil), checklist.Required...),
		Recommended: append([]ChecklistGroupItem[Reason, Hint](nil), checklist.Recommended...),
	}
	if len(out.Required) == 0 && len(out.Recommended) == 0 {
		return nil
	}
	return out
}

func BuildSuccessSuggestedFollowUpRevision(mode string, pkg *sheinpub.Package) *EditorRevisionSkeleton {
	if pkg == nil {
		return nil
	}
	skeleton := BuildMinimalRevisionSkeleton(BuildEditorRevisionSkeleton(
		pkg,
		BuildCategoryResolutionPatch(pkg),
		BuildAttributeResolutionPatch(pkg),
		BuildSaleAttributeResolutionPatch(pkg),
		BuildEditorSKCPatches(pkg),
	))
	if skeleton == nil {
		return nil
	}
	skeleton.Actor = "desktop-client"
	if mode == SuccessModeRestore {
		skeleton.Reason = "follow-up after restore"
	} else {
		skeleton.Reason = "follow-up after apply"
	}
	return skeleton
}

func BuildSuccessFollowUpOverview[Item any](mode string, summary *SuccessStatusSummary, messages *SuccessMessages, checklist *SuccessFollowUpChecklist[Item], nextActions []string) *SuccessFollowUpOverview {
	overview := &SuccessFollowUpOverview{
		Status:      firstNonEmpty(summaryStatus(summary), "ready"),
		Headline:    buildSuccessFollowUpHeadline(mode, summary),
		Subheadline: firstNonEmpty(summarySubheadline(summary), messageDescription(messages)),
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

func BuildSuccessSummaryCard(mode, headline, relationText string, changeCount int, messages *SuccessMessages, appliedChanges *RevisionDiffPreview, summary *SuccessStatusSummary, recommendedView *SuccessRecommendedView, nextActions []string) *SuccessSummaryCard {
	var highlights []string
	switch mode {
	case SuccessModeRestore:
		highlights = append([]string{relationText}, summaryHighlights(summary)...)
		highlights = append(highlights, messageWarnings(messages)...)
		return &SuccessSummaryCard{
			Status:        firstNonEmpty(summaryStatus(summary), "ready"),
			Title:         firstNonEmpty(messageTitle(messages), headline, "历史版本已恢复"),
			Subtitle:      firstNonEmpty(summarySubheadline(summary), messageDescription(messages), relationText),
			PrimaryAction: firstNonEmpty(firstAction(nextActions), "继续提交流程"),
			PrimaryView:   recommendedViewValue(recommendedView),
			Highlights:    uniqueStrings(filterNonEmptyStrings(highlights)),
		}
	default:
		highlights = append(buildApplyHighlights(changeCount, appliedChanges), summaryHighlights(summary)...)
		highlights = append(highlights, messageWarnings(messages)...)
		return &SuccessSummaryCard{
			Status:        firstNonEmpty(summaryStatus(summary), "ready"),
			Title:         firstNonEmpty(headline, "资料已更新"),
			Subtitle:      firstNonEmpty(messageDescription(messages), summarySubheadline(summary), buildApplySubtitle(changeCount)),
			PrimaryAction: firstNonEmpty(buildApplyPrimaryAction(summary), "继续提交流程"),
			PrimaryView:   recommendedViewValue(recommendedView),
			Highlights:    uniqueStrings(filterNonEmptyStrings(highlights)),
		}
	}
}

func BuildSuccessPresentation(scene string, nextActions []string, messages *SuccessMessages, recommendedView *SuccessRecommendedView, summaryCard *SuccessSummaryCard) *SuccessInteractionPresentation {
	if scene == "" && len(nextActions) == 0 && messages == nil && recommendedView == nil && summaryCard == nil {
		return nil
	}
	return &SuccessInteractionPresentation{
		Scene:           scene,
		NextActions:     append([]string(nil), nextActions...),
		Messages:        cloneSuccessMessages(messages),
		RecommendedView: cloneSuccessRecommendedView(recommendedView),
		SummaryCard:     cloneSuccessSummaryCard(summaryCard),
	}
}

func BuildSuccessPayload[Item any](mode, actionType, headline, sourceRevisionID, relationText string, changeCount int, statusSummary *SuccessStatusSummary, presentation *SuccessInteractionPresentation, followUpChecklist *SuccessFollowUpChecklist[Item], followUpOverview *SuccessFollowUpOverview, suggestedFollowUpRevision *EditorRevisionSkeleton, appliedChanges *RevisionDiffPreview) *SuccessPayload[Item] {
	return &SuccessPayload[Item]{
		Mode: mode,
		Core: &SuccessCoreData[Item]{
			ActionType:                actionType,
			Headline:                  headline,
			ChangeCount:               changeCount,
			SourceRevisionID:          sourceRevisionID,
			RelationText:              relationText,
			StatusSummary:             cloneSuccessStatusSummary(statusSummary),
			FollowUpChecklist:         cloneSuccessFollowUpChecklist(followUpChecklist),
			FollowUpOverview:          cloneSuccessFollowUpOverview(followUpOverview),
			SuggestedFollowUpRevision: CloneEditorRevisionSkeleton(suggestedFollowUpRevision),
			AppliedChanges:            cloneRevisionDiffPreview(appliedChanges),
		},
		Presentation: presentation,
	}
}

func toSubmitStateInput[Reason any, Hint any](readiness *SubmitReadiness[Reason, Hint]) *SubmitStateInput {
	if readiness == nil {
		return nil
	}
	out := &SubmitStateInput{
		Status:  readiness.Status,
		Ready:   readiness.Ready,
		Summary: append([]string(nil), readiness.Summary...),
	}
	for _, item := range readiness.BlockingItems {
		out.BlockingItems = append(out.BlockingItems, ActionItem{Key: item.Key, SuggestedAction: item.SuggestedAction})
	}
	for _, item := range readiness.WarningItems {
		out.WarningItems = append(out.WarningItems, ActionItem{Key: item.Key, SuggestedAction: item.SuggestedAction})
	}
	return out
}

func buildApplyHighlights(changeCount int, appliedChanges *RevisionDiffPreview) []string {
	if appliedChanges == nil {
		return nil
	}
	highlights := make([]string, 0, len(appliedChanges.Changes))
	for i, change := range appliedChanges.Changes {
		if i >= 3 {
			break
		}
		highlights = append(highlights, firstNonEmpty(change.Label, change.FieldPath))
	}
	if changeCount > len(highlights) {
		highlights = append(highlights, "已保存字段变更")
	}
	return highlights
}

func buildApplySubtitle(changeCount int) string {
	if changeCount > 0 {
		return "本次共更新 " + formatInt(changeCount) + " 个字段。"
	}
	return "资料已保存。"
}

func buildApplyPrimaryAction(summary *SuccessStatusSummary) string {
	switch {
	case summary == nil:
		return ""
	case summary.Status == "blocked":
		return "继续补齐资料"
	case summary.NeedsReview:
		return "前往检查"
	default:
		return "继续提交流程"
	}
}

func buildSuccessFollowUpHeadline(mode string, summary *SuccessStatusSummary) string {
	switch {
	case summary == nil:
		return ""
	case summary.Status == "blocked":
		if mode == SuccessModeRestore {
			return "恢复后仍需继续补齐资料"
		}
		return "保存后仍需继续补齐资料"
	case summary.NeedsReview:
		if mode == SuccessModeRestore {
			return "恢复后建议先继续检查"
		}
		return "保存后建议先继续检查"
	default:
		if mode == SuccessModeRestore {
			return "恢复后可以继续提交流程"
		}
		return "保存后可以继续提交流程"
	}
}

func defaultSuccessTitle(mode string) string {
	if mode == SuccessModeRestore {
		return "历史版本已恢复"
	}
	return "资料已更新"
}

func defaultSuccessLabel(mode string) string {
	if mode == SuccessModeRestore {
		return "恢复成功"
	}
	return "保存成功"
}

func defaultSuccessWarningTitle(mode string) string {
	if mode == SuccessModeRestore {
		return "恢复后仍建议继续确认"
	}
	return "保存后仍建议继续确认"
}

func defaultSuccessEditorReason(mode string) string {
	if mode == SuccessModeRestore {
		return "恢复后仍有关键字段待补齐，建议继续编辑。"
	}
	return "保存后仍有关键字段待补齐，建议继续编辑。"
}

func defaultSuccessInspectionReason(mode string) string {
	if mode == SuccessModeRestore {
		return "恢复后仍有待确认项，建议先检查。"
	}
	return "保存后仍有待确认项，建议先检查。"
}

func defaultSuccessSubmitReason(mode string) string {
	if mode == SuccessModeRestore {
		return "恢复后可以直接继续提交流程。"
	}
	return "保存后可以直接继续提交流程。"
}

func summaryStatus(summary *SuccessStatusSummary) string {
	if summary == nil {
		return ""
	}
	return summary.Status
}

func summarySubheadline(summary *SuccessStatusSummary) string {
	if summary == nil {
		return ""
	}
	return summary.Subheadline
}

func summaryHighlights(summary *SuccessStatusSummary) []string {
	if summary == nil {
		return nil
	}
	return append([]string(nil), summary.Highlights...)
}

func messageTitle(messages *SuccessMessages) string {
	if messages == nil {
		return ""
	}
	return messages.Title
}

func messageDescription(messages *SuccessMessages) string {
	if messages == nil {
		return ""
	}
	return messages.Description
}

func messageWarnings(messages *SuccessMessages) []string {
	if messages == nil {
		return nil
	}
	return append([]string(nil), messages.WarningSummaries...)
}

func recommendedViewValue(view *SuccessRecommendedView) string {
	if view == nil {
		return ""
	}
	return view.View
}

func firstAction(actions []string) string {
	for _, action := range actions {
		if action != "" {
			return action
		}
	}
	return ""
}

func cloneSuccessStatusSummary(src *SuccessStatusSummary) *SuccessStatusSummary {
	if src == nil {
		return nil
	}
	return &SuccessStatusSummary{
		Status:        src.Status,
		Headline:      src.Headline,
		Subheadline:   src.Subheadline,
		NeedsReview:   src.NeedsReview,
		BlockingCount: src.BlockingCount,
		WarningCount:  src.WarningCount,
		Highlights:    append([]string(nil), src.Highlights...),
	}
}

func cloneSuccessMessages(src *SuccessMessages) *SuccessMessages {
	if src == nil {
		return nil
	}
	return &SuccessMessages{
		Title:            src.Title,
		Description:      src.Description,
		SuccessLabel:     src.SuccessLabel,
		WarningTitle:     src.WarningTitle,
		WarningSummaries: append([]string(nil), src.WarningSummaries...),
	}
}

func cloneSuccessRecommendedView(src *SuccessRecommendedView) *SuccessRecommendedView {
	if src == nil {
		return nil
	}
	return &SuccessRecommendedView{
		View:   src.View,
		Reason: src.Reason,
	}
}

func cloneSuccessFollowUpChecklist[Item any](src *SuccessFollowUpChecklist[Item]) *SuccessFollowUpChecklist[Item] {
	if src == nil {
		return nil
	}
	return &SuccessFollowUpChecklist[Item]{
		Required:    append([]Item(nil), src.Required...),
		Recommended: append([]Item(nil), src.Recommended...),
	}
}

func cloneSuccessFollowUpOverview(src *SuccessFollowUpOverview) *SuccessFollowUpOverview {
	if src == nil {
		return nil
	}
	return &SuccessFollowUpOverview{
		Status:           src.Status,
		Headline:         src.Headline,
		Subheadline:      src.Subheadline,
		RequiredCount:    src.RequiredCount,
		RecommendedCount: src.RecommendedCount,
		NextActions:      append([]string(nil), src.NextActions...),
	}
}

func cloneSuccessSummaryCard(src *SuccessSummaryCard) *SuccessSummaryCard {
	if src == nil {
		return nil
	}
	return &SuccessSummaryCard{
		Status:        src.Status,
		Title:         src.Title,
		Subtitle:      src.Subtitle,
		PrimaryAction: src.PrimaryAction,
		PrimaryView:   src.PrimaryView,
		Highlights:    append([]string(nil), src.Highlights...),
	}
}

func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {
	if src == nil {
		return nil
	}
	return &RevisionDiffPreview{
		ChangeCount: src.ChangeCount,
		Changes:     append([]RevisionFieldChange(nil), src.Changes...),
	}
}
