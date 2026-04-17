package listingkit

func buildRevisionApplyResultMessages(headline string, changeCount int, statusSummary *RevisionStatusSummary) *RevisionResultMessages {
	return buildRevisionSuccessMessages(
		revisionSuccessModeApply,
		headline,
		changeCount,
		"",
		statusSummary,
	)
}
