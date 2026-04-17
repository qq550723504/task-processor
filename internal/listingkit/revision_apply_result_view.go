package listingkit

func buildRevisionApplyRecommendedView(listingResult *ListingKitResult) *RevisionRecommendedView {
	return buildRevisionSuccessRecommendedView(revisionSuccessModeApply, listingResult, buildRevisionRestoreStatusSummary(listingResult))
}
