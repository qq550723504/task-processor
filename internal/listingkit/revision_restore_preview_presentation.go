package listingkit

func buildRevisionRestorePreviewPresentation(
	record *ListingKitRevisionRecord,
	context *RevisionHistoryRestoreContext,
	safety *RevisionHistoryRestoreSafety,
	comparePreview *RevisionHistoryComparePreview,
) *RevisionInteractionPresentation {
	data := buildRevisionHistoryRestorePresentationData(record, context, safety, comparePreview)
	if data == nil {
		return nil
	}

	summaryCard := &RevisionSuccessSummaryCard{
		Status:        data.Status,
		Title:         data.Headline,
		Subtitle:      data.Subheadline,
		PrimaryAction: data.PrimaryAction,
		Highlights:    append([]string(nil), data.Highlights...),
	}

	return buildRevisionInteractionPresentation(
		revisionPresentationSceneRestorePreview,
		append([]string(nil), data.NextActions...),
		&RevisionResultMessages{
			Title:            data.Title,
			Description:      data.Description,
			SuccessLabel:     data.ConfirmLabel,
			WarningTitle:     data.WarningTitle,
			WarningSummaries: append([]string(nil), data.WarningSummaries...),
		},
		convertHistoryRestoreRecommendedView(data.RecommendedView),
		summaryCard,
	)
}
