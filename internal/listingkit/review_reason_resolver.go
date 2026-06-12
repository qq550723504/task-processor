package listingkit

type reviewReasonSources struct {
	WorkflowReasons    []string
	ResultReasons      []string
	SummaryNeedsReview bool
	SummaryWarnings    []string
	PodBlocked         bool
	PodFailureReason   string
	PlatformReasons    []string
}

func resolveReviewReasons(sources reviewReasonSources) []string {
	if reasons := normalizeReviewReasons(sources.WorkflowReasons); len(reasons) > 0 {
		return reasons
	}
	if reasons := normalizeReviewReasons(sources.ResultReasons); len(reasons) > 0 {
		return reasons
	}
	if sources.SummaryNeedsReview {
		if reasons := normalizeReviewReasons(sources.SummaryWarnings); len(reasons) > 0 {
			return reasons
		}
	}
	if sources.PodBlocked {
		if reasons := normalizeReviewReasons([]string{sources.PodFailureReason}); len(reasons) > 0 {
			return reasons
		}
	}
	return normalizeReviewReasons(sources.PlatformReasons)
}
