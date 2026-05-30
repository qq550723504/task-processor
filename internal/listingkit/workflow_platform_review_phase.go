package listingkit

type platformReviewPhase struct{}

func buildPlatformReviewPhase() *platformReviewPhase {
	return &platformReviewPhase{}
}

func (p *platformReviewPhase) run(
	final *ListingKitResult,
	snapshot *StandardProductSnapshot,
) {
	if final.Summary == nil {
		final.Summary = &GenerationSummary{}
	}
	if snapshot != nil && snapshot.Summary != nil {
		final.Summary.Warnings = uniqueStrings(append(final.Summary.Warnings, snapshot.Summary.Warnings...))
	}

	sheinReviewStage := newWorkflowRecorder(final).Start("shein_review", "")
	applySheinInspectionReviewToSummary(final)
	applySheinVariantCoverageReviewToSummary(final)
	addSheinReviewWorkflowIssues(final)
	sheinReviewStage.Complete()
}
