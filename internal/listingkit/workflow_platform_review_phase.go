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
	withSheinVariantCoverageReviewSuppressed(final, func() {
		addSheinReviewWorkflowIssues(final)
	})
	sheinReviewStage.Complete()
}

func withSheinVariantCoverageReviewSuppressed(result *ListingKitResult, fn func()) {
	if result == nil || fn == nil {
		return
	}
	coverageWarning, blocked := sheinVariantImageCoverageStatus(result.Shein)
	if !blocked || coverageWarning == "" {
		fn()
		return
	}
	originalSummaryWarnings := append([]string(nil), result.Summary.Warnings...)
	originalReviewReasons := append([]string(nil), result.ReviewReasons...)
	originalReviewNotes := append([]string(nil), result.Shein.ReviewNotes...)
	result.Summary.Warnings = filterStrings(result.Summary.Warnings, coverageWarning)
	result.ReviewReasons = filterStrings(result.ReviewReasons, coverageWarning)
	result.Shein.ReviewNotes = filterStrings(result.Shein.ReviewNotes, coverageWarning)
	fn()
	result.Summary.Warnings = originalSummaryWarnings
	result.ReviewReasons = originalReviewReasons
	result.Shein.ReviewNotes = originalReviewNotes
}

func filterStrings(items []string, skip string) []string {
	if len(items) == 0 {
		return nil
	}
	skip = firstNonEmpty(skip)
	if skip == "" {
		return append([]string(nil), items...)
	}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if item == skip {
			continue
		}
		filtered = append(filtered, item)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
