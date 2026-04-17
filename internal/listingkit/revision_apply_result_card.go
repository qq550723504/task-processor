package listingkit

func buildRevisionApplySummaryCard(
	headline string,
	changeCount int,
	messages *RevisionResultMessages,
	appliedChanges *RevisionDiffPreview,
	listingResult *ListingKitResult,
) *RevisionSuccessSummaryCard {
	data := buildRevisionApplySummaryCardData(headline, changeCount, messages, appliedChanges, listingResult)
	return &RevisionSuccessSummaryCard{
		Status:        data.Status,
		Title:         data.Title,
		Subtitle:      data.Subtitle,
		PrimaryAction: data.PrimaryAction,
		PrimaryView:   data.PrimaryView,
		Highlights:    data.Highlights,
	}
}

func buildRevisionApplySubtitle(changeCount int) string {
	if changeCount > 0 {
		return "本次共更新 " + formatInt(changeCount) + " 个字段。"
	}
	return "资料已保存。"
}

func buildRevisionApplyPrimaryAction(summary *RevisionStatusSummary) string {
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

func buildRevisionApplyHighlights(changeCount int, appliedChanges *RevisionDiffPreview) []string {
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
