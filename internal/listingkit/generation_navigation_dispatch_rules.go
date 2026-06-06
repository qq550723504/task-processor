package listingkit

func applyGenerationNavigationDispatchExecutionRules(plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	if plan == nil || execution == nil {
		return
	}
	markGenerationNavigationDispatchWinners(execution)
	applyGenerationNavigationDispatchFallbacks(plan, execution)
	for index := range execution.Steps {
		applyGenerationNavigationDispatchRecovery(&execution.Steps[index])
	}
}

func markGenerationNavigationDispatchWinners(execution *GenerationNavigationDispatchExecution) {
	if execution == nil {
		return
	}
	if winner := bestGenerationNavigationDispatchExecutionStep(execution, "queue"); winner != nil {
		winner.Winner = true
	}
	if winner := bestGenerationNavigationDispatchExecutionStep(execution, "session"); winner != nil {
		winner.Winner = true
	}
	if winner := bestGenerationNavigationDispatchExecutionStep(execution, "preview"); winner != nil {
		winner.Winner = true
	}
}

func applyGenerationNavigationDispatchFallbacks(plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	if plan == nil || execution == nil {
		return
	}
	previewWinner := bestGenerationNavigationDispatchExecutionStep(execution, "preview")
	sessionWinner := bestGenerationNavigationDispatchExecutionStep(execution, "session")
	queueWinner := bestGenerationNavigationDispatchExecutionStep(execution, "queue")

	if previewWinner == nil && sessionWinner != nil && hasGenerationNavigationDispatchFailure(execution, "preview") {
		sessionWinner.FallbackApplied = true
		sessionWinner.FallbackReason = "preview_failed_use_session_focus"
		sessionWinner.FallbackCandidate = true
		sessionWinner.FallbackSourceKind = "session"
		markGenerationNavigationDispatchFallbackCandidates(execution, "preview", "session", sessionWinner.FallbackReason)
	}
	if queueWinner == nil && sessionWinner != nil && sessionWinner.ReviewSession != nil && sessionWinner.ReviewSession.Session != nil && sessionWinner.ReviewSession.Session.Queue != nil && hasGenerationNavigationDispatchFailure(execution, "queue") {
		sessionWinner.FallbackApplied = true
		sessionWinner.FallbackCandidate = true
		sessionWinner.FallbackSourceKind = "session"
		if sessionWinner.FallbackReason == "" {
			sessionWinner.FallbackReason = "queue_failed_use_session_summary"
		}
		markGenerationNavigationDispatchFallbackCandidates(execution, "queue", "session", sessionWinner.FallbackReason)
	}
}

func markGenerationNavigationDispatchFallbackCandidates(execution *GenerationNavigationDispatchExecution, kind string, sourceKind string, reason string) {
	if execution == nil {
		return
	}
	for index := range execution.Steps {
		step := &execution.Steps[index]
		if step.Kind != kind || step.Status != "failed" {
			continue
		}
		step.FallbackCandidate = true
		step.FallbackSourceKind = sourceKind
		if step.FallbackReason == "" {
			step.FallbackReason = reason
		}
	}
}

func bestGenerationNavigationDispatchExecutionStep(execution *GenerationNavigationDispatchExecution, kind string) *GenerationNavigationDispatchExecutionStep {
	if execution == nil {
		return nil
	}
	var winner *GenerationNavigationDispatchExecutionStep
	bestScore := -1
	for index := range execution.Steps {
		step := &execution.Steps[index]
		if step.Kind != kind {
			continue
		}
		score := generationNavigationDispatchStepWinnerScore(step)
		if score > bestScore {
			bestScore = score
			winner = step
		}
	}
	if bestScore <= 0 {
		return nil
	}
	return winner
}

func bestGenerationNavigationDispatchExecutionStepWithIndex(execution *GenerationNavigationDispatchExecution, kind string) (*GenerationNavigationDispatchExecutionStep, int) {
	if execution == nil {
		return nil, -1
	}
	bestScore := -1
	bestIndex := -1
	for index := range execution.Steps {
		step := &execution.Steps[index]
		if step.Kind != kind {
			continue
		}
		score := generationNavigationDispatchStepWinnerScore(step)
		if score > bestScore {
			bestScore = score
			bestIndex = index
		}
	}
	if bestIndex < 0 || bestScore <= 0 {
		return nil, -1
	}
	return &execution.Steps[bestIndex], bestIndex
}

func generationNavigationDispatchStepWinnerScore(step *GenerationNavigationDispatchExecutionStep) int {
	if step == nil {
		return 0
	}
	switch step.Status {
	case "completed":
		if step.Kind == "preview" && step.ReviewPreview == nil {
			return 0
		}
		if step.Kind == "session" && step.ReviewSession == nil {
			return 0
		}
		if step.Kind == "queue" && step.Queue == nil {
			return 0
		}
		return 4
	case "not_modified":
		if step.Kind == "preview" && step.ReviewPreview == nil {
			return 0
		}
		if step.Kind == "session" && step.ReviewSession == nil {
			return 0
		}
		if step.Kind == "queue" && step.Queue == nil {
			return 0
		}
		return 3
	case "deduplicated":
		return 2
	default:
		return 0
	}
}

func hasGenerationNavigationDispatchFailure(execution *GenerationNavigationDispatchExecution, kind string) bool {
	if execution == nil {
		return false
	}
	for _, step := range execution.Steps {
		if step.Kind == kind && step.Status == "failed" {
			return true
		}
	}
	return false
}

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
