package listingkit

func buildRevisionRestoreRecommendedView(result *ListingKitResult, summary *RevisionStatusSummary) *RevisionRecommendedView {
	return buildRevisionSuccessRecommendedView(revisionSuccessModeRestore, result, summary)
}
