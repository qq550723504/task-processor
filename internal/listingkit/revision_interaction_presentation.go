package listingkit

const (
	revisionPresentationSceneApplySuccess   = "apply_success"
	revisionPresentationSceneRestoreSuccess = "restore_success"
	revisionPresentationSceneRestorePreview = "restore_preview"
)

func buildRevisionInteractionPresentation(
	scene string,
	nextActions []string,
	messages *RevisionResultMessages,
	recommendedView *RevisionRecommendedView,
	summaryCard *RevisionSuccessSummaryCard,
) *RevisionInteractionPresentation {
	if scene == "" && len(nextActions) == 0 && messages == nil && recommendedView == nil && summaryCard == nil {
		return nil
	}
	return &RevisionInteractionPresentation{
		Scene:           scene,
		NextActions:     append([]string(nil), nextActions...),
		Messages:        messages,
		RecommendedView: recommendedView,
		SummaryCard:     cloneRevisionSuccessSummaryCard(summaryCard),
	}
}

func buildRevisionRestorePreviewRecommendedView(safety *RevisionHistoryRestoreSafety) *RevisionRecommendedView {
	if safety == nil {
		return nil
	}
	switch {
	case !safety.CanRestore:
		return &RevisionRecommendedView{
			View:   "inspection",
			Reason: "当前恢复条件还不完整，建议先检查阻塞项。",
		}
	case len(safety.RestoreWarnings) > 0:
		return &RevisionRecommendedView{
			View:   "inspection",
			Reason: "恢复前还有提醒项，建议先确认影响范围。",
		}
	default:
		return &RevisionRecommendedView{
			View:   "submit",
			Reason: "当前可以直接执行恢复。",
		}
	}
}
