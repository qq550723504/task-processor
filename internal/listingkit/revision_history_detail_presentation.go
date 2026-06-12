package listingkit

import sheinworkspace "task-processor/internal/listingkit/workspace/shein"

func buildRevisionRestorePreviewPresentationValue(
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

	return sheinworkspace.BuildSuccessPresentation(
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

func restoreDetailContextValue(detail *revisionHistoryRestoreDetailData) *RevisionHistoryRestoreContext {
	if detail == nil {
		return nil
	}
	return detail.Context
}

func restoreDetailSafetyValue(detail *revisionHistoryRestoreDetailData) *RevisionHistoryRestoreSafety {
	if detail == nil {
		return nil
	}
	return detail.Safety
}
