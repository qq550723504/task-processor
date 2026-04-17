package listingkit

func buildRevisionRestoreResultMessages(headline string, restoredFieldCount int, sourceRevisionID string, statusSummary *RevisionStatusSummary) *RevisionResultMessages {
	return buildRevisionSuccessMessages(
		revisionSuccessModeRestore,
		headline,
		restoredFieldCount,
		sourceRevisionID,
		statusSummary,
	)
}
