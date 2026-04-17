package listingkit

func buildRevisionSuccessPayloadForApply(
	actionType string,
	headline string,
	changeCount int,
	statusSummary *RevisionStatusSummary,
	presentation *RevisionInteractionPresentation,
	followUpChecklist *RevisionFollowUpChecklist,
	followUpOverview *RevisionFollowUpOverview,
	suggestedFollowUpRevision *SheinEditorRevisionSkeleton,
	appliedChanges *RevisionDiffPreview,
) *RevisionSuccessPayload {
	return &RevisionSuccessPayload{
		Mode: string(revisionSuccessModeApply),
		Core: &RevisionSuccessCoreData{
			ActionType:                actionType,
			Headline:                  headline,
			ChangeCount:               changeCount,
			StatusSummary:             statusSummary,
			FollowUpChecklist:         followUpChecklist,
			FollowUpOverview:          followUpOverview,
			SuggestedFollowUpRevision: suggestedFollowUpRevision,
			AppliedChanges:            appliedChanges,
		},
		Presentation: presentation,
	}
}

func buildRevisionSuccessPayloadForRestore(
	actionType string,
	headline string,
	sourceRevisionID string,
	relationText string,
	changeCount int,
	statusSummary *RevisionStatusSummary,
	presentation *RevisionInteractionPresentation,
	followUpChecklist *RevisionFollowUpChecklist,
	followUpOverview *RevisionFollowUpOverview,
	suggestedFollowUpRevision *SheinEditorRevisionSkeleton,
	appliedChanges *RevisionDiffPreview,
) *RevisionSuccessPayload {
	return &RevisionSuccessPayload{
		Mode: string(revisionSuccessModeRestore),
		Core: &RevisionSuccessCoreData{
			ActionType:                actionType,
			Headline:                  headline,
			ChangeCount:               changeCount,
			SourceRevisionID:          sourceRevisionID,
			RelationText:              relationText,
			StatusSummary:             statusSummary,
			FollowUpChecklist:         followUpChecklist,
			FollowUpOverview:          followUpOverview,
			SuggestedFollowUpRevision: suggestedFollowUpRevision,
			AppliedChanges:            appliedChanges,
		},
		Presentation: presentation,
	}
}

func cloneRevisionSuccessSummaryCard(card *RevisionSuccessSummaryCard) *RevisionSuccessSummaryCard {
	if card == nil {
		return nil
	}
	return &RevisionSuccessSummaryCard{
		Status:        card.Status,
		Title:         card.Title,
		Subtitle:      card.Subtitle,
		PrimaryAction: card.PrimaryAction,
		PrimaryView:   card.PrimaryView,
		Highlights:    append([]string(nil), card.Highlights...),
	}
}
