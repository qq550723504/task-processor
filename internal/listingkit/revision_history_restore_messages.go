package listingkit

type RevisionHistoryRestoreMessages struct {
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	ConfirmLabel     string   `json:"confirm_label,omitempty"`
	CancelLabel      string   `json:"cancel_label,omitempty"`
	WarningTitle     string   `json:"warning_title,omitempty"`
	WarningSummaries []string `json:"warning_summaries,omitempty"`
}

func buildRevisionHistoryRestoreMessages(record *ListingKitRevisionRecord, context *RevisionHistoryRestoreContext, safety *RevisionHistoryRestoreSafety, overview *RevisionHistoryRestoreOverview) *RevisionHistoryRestoreMessages {
	if record == nil && context == nil && safety == nil && overview == nil {
		return nil
	}

	msg := &RevisionHistoryRestoreMessages{
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
