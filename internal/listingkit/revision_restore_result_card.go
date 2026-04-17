package listingkit

func buildRevisionRestoreSummaryCard(
	headline string,
	relationText string,
	nextActions []string,
	statusSummary *RevisionStatusSummary,
	messages *RevisionResultMessages,
	recommendedView *RevisionRecommendedView,
) *RevisionSuccessSummaryCard {
	data := buildRevisionRestoreSummaryCardData(headline, relationText, nextActions, statusSummary, messages, recommendedView)
	return &RevisionSuccessSummaryCard{
		Status:        data.Status,
		Title:         data.Title,
		Subtitle:      data.Subtitle,
		PrimaryAction: data.PrimaryAction,
		PrimaryView:   data.PrimaryView,
		Highlights:    data.Highlights,
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
