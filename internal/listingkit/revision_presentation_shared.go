package listingkit

type revisionPresentationData struct {
	Status        string
	Title         string
	Subtitle      string
	PrimaryAction string
	PrimaryView   string
	NextActions   []string
	Highlights    []string
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

func summaryStatusValue(summary *RevisionStatusSummary) string {
	if summary == nil {
		return ""
	}
	return summary.Status
}

func summarySubheadlineValue(summary *RevisionStatusSummary) string {
	if summary == nil {
		return ""
	}
	return summary.Subheadline
}

func summaryHighlightsValue(summary *RevisionStatusSummary) []string {
	if summary == nil {
		return nil
	}
	return append([]string(nil), summary.Highlights...)
}

func messageTitleValue(messages *RevisionResultMessages) string {
	if messages == nil {
		return ""
	}
	return messages.Title
}

func messageDescriptionValue(messages *RevisionResultMessages) string {
	if messages == nil {
		return ""
	}
	return messages.Description
}

func messageWarningsValue(messages *RevisionResultMessages) []string {
	if messages == nil {
		return nil
	}
	return append([]string(nil), messages.WarningSummaries...)
}

func recommendedViewValue(view *RevisionRecommendedView) string {
	if view == nil {
		return ""
	}
	return view.View
}

func firstString(values []string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeRevisionPresentation(data revisionPresentationData) revisionPresentationData {
	data.NextActions = uniqueStrings(filterNonEmptyStrings(data.NextActions))
	data.Highlights = uniqueStrings(filterNonEmptyStrings(data.Highlights))
	return data
}

func buildRevisionApplySummaryCardData(
	headline string,
	changeCount int,
	messages *RevisionResultMessages,
	appliedChanges *RevisionDiffPreview,
	listingResult *ListingKitResult,
) revisionPresentationData {
	statusSummary := buildRevisionSuccessStatusSummary(listingResult)
	recommendedView := buildRevisionSuccessRecommendedView(revisionSuccessModeApply, listingResult, statusSummary)
	highlights := append(buildRevisionApplyHighlights(changeCount, appliedChanges), summaryHighlightsValue(statusSummary)...)
	highlights = append(highlights, messageWarningsValue(messages)...)

	return normalizeRevisionPresentation(revisionPresentationData{
		Status: firstNonEmpty(summaryStatusValue(statusSummary), "ready"),
		Title:  firstNonEmpty(headline, "资料已更新"),
		Subtitle: firstNonEmpty(
			messageDescriptionValue(messages),
			summarySubheadlineValue(statusSummary),
			buildRevisionApplySubtitle(changeCount),
		),
		PrimaryAction: firstNonEmpty(buildRevisionApplyPrimaryAction(statusSummary), "继续提交流程"),
		PrimaryView:   recommendedViewValue(recommendedView),
		Highlights:    highlights,
	})
}

func buildRevisionRestoreSummaryCardData(
	headline string,
	relationText string,
	nextActions []string,
	statusSummary *RevisionStatusSummary,
	messages *RevisionResultMessages,
	recommendedView *RevisionRecommendedView,
) revisionPresentationData {
	highlights := append([]string{relationText}, summaryHighlightsValue(statusSummary)...)
	highlights = append(highlights, messageWarningsValue(messages)...)

	return normalizeRevisionPresentation(revisionPresentationData{
		Status: firstNonEmpty(summaryStatusValue(statusSummary), "ready"),
		Title: firstNonEmpty(
			messageTitleValue(messages),
			headline,
			"历史版本已恢复",
		),
		Subtitle: firstNonEmpty(
			summarySubheadlineValue(statusSummary),
			messageDescriptionValue(messages),
			relationText,
		),
		PrimaryAction: firstNonEmpty(firstString(nextActions), "继续提交流程"),
		PrimaryView:   recommendedViewValue(recommendedView),
		Highlights:    highlights,
	})
}

func buildRevisionHistoryRestoreOverviewData(
	record *ListingKitRevisionRecord,
	safety *RevisionHistoryRestoreSafety,
	comparePreview *RevisionHistoryComparePreview,
) revisionPresentationData {
	overview := buildRevisionHistoryRestoreOverview(record, safety, comparePreview)
	if overview == nil {
		return revisionPresentationData{}
	}
	return normalizeRevisionPresentation(revisionPresentationData{
		Status:        overview.Status,
		Title:         overview.Headline,
		Subtitle:      overview.Subheadline,
		PrimaryAction: overview.PrimaryAction,
		NextActions:   append([]string(nil), overview.NextActions...),
		Highlights:    append([]string(nil), overview.Highlights...),
	})
}
