package listingkit

func applyGenerationNavigationDispatchRecovery(step *GenerationNavigationDispatchExecutionStep) {
	if step == nil {
		return
	}
	if step.FallbackApplied || step.FallbackCandidate {
		step.Retryable = false
		step.RetryHint = "review_fallback"
		return
	}
	switch step.Status {
	case "failed":
		switch step.ErrorKind {
		case "internal":
			step.Retryable = true
			step.RetryHint = "retry_dispatch"
		case "conflict":
			step.Retryable = false
			step.RetryHint = "refresh_revision"
		case "not_found":
			step.Retryable = false
			step.RetryHint = "wait_for_generation"
		default:
			step.Retryable = false
			step.RetryHint = "no_retry"
		}
	case "completed", "not_modified", "deduplicated", "skipped":
		step.Retryable = false
		step.RetryHint = "no_retry"
	default:
		step.Retryable = false
		step.RetryHint = "no_retry"
	}
}
