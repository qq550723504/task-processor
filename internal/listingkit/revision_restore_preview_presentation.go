package listingkit

func buildRevisionRestorePreviewPresentation(
	record *ListingKitRevisionRecord,
	context *RevisionHistoryRestoreContext,
	safety *RevisionHistoryRestoreSafety,
	comparePreview *RevisionHistoryComparePreview,
) *RevisionInteractionPresentation {
	overview := buildRevisionHistoryRestoreOverview(record, safety, comparePreview)
	messages := buildRevisionHistoryRestoreMessages(record, context, safety, overview)
	if overview == nil && messages == nil {
		return nil
	}

	var summaryCard *RevisionSuccessSummaryCard
	if overview != nil {
		summaryCard = &RevisionSuccessSummaryCard{
			Status:        overview.Status,
			Title:         overview.Headline,
			Subtitle:      overview.Subheadline,
			PrimaryAction: overview.PrimaryAction,
			Highlights:    append([]string(nil), overview.Highlights...),
		}
	}

	return buildRevisionInteractionPresentation(
		revisionPresentationSceneRestorePreview,
		overviewNextActions(overview),
		convertRestoreMessages(messages),
		buildRevisionRestorePreviewRecommendedView(safety),
		summaryCard,
	)
}

func overviewNextActions(overview *RevisionHistoryRestoreOverview) []string {
	if overview == nil {
		return nil
	}
	return append([]string(nil), overview.NextActions...)
}

func convertRestoreMessages(messages *RevisionHistoryRestoreMessages) *RevisionResultMessages {
	if messages == nil {
		return nil
	}
	return &RevisionResultMessages{
		Title:            messages.Title,
		Description:      messages.Description,
		SuccessLabel:     messages.ConfirmLabel,
		WarningTitle:     messages.WarningTitle,
		WarningSummaries: append([]string(nil), messages.WarningSummaries...),
	}
}
