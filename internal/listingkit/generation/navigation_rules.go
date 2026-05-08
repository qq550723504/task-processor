package generation

func NavigationRefreshScope(resourceKind string) string {
	switch resourceKind {
	case "generation_action":
		return "mutation"
	case "review_preview":
		return "focused_read"
	case "generation_queue":
		return "collection_read"
	default:
		return "panel_read"
	}
}

func NavigationInvalidates(resourceKind string) []string {
	switch resourceKind {
	case "generation_action":
		return []string{"review_session", "review_preview", "generation_queue"}
	case "review_preview":
		return []string{"review_preview"}
	case "generation_queue":
		return []string{"generation_queue"}
	default:
		return []string{"review_session"}
	}
}

func NavigationCachePolicy(resourceKind string) string {
	switch resourceKind {
	case "generation_action":
		return "network_only"
	case "generation_queue":
		return "revalidate"
	case "review_preview", "review_session":
		return "stale_while_revalidate"
	default:
		return "revalidate"
	}
}

func NavigationRevalidateAfterAction(resourceKind string) bool {
	return resourceKind == "generation_action"
}

func NavigationDispatchStrategy(resourceKind string, readCount int) string {
	switch resourceKind {
	case "generation_action":
		return "mutation_then_refresh"
	case "generation_queue", "review_preview", "review_session":
		if readCount <= 1 {
			return "single_read"
		}
		return "fanout_read"
	default:
		if readCount <= 1 {
			return "single_read"
		}
		return "fanout_read"
	}
}

func NavigationDispatchStopOnNotModified(resourceKind string, readCount int) bool {
	return readCount <= 1 && resourceKind != "generation_action"
}

func NavigationDispatchStopOnFirstSuccess(resourceKind string, readCount int) bool {
	return readCount <= 1 && resourceKind != "generation_action"
}

func NavigationDispatchStopOnError(readCount int) bool {
	return readCount <= 1
}

func NavigationDispatchFallbackStrategy(resourceKind string, readCount int) string {
	switch resourceKind {
	case "generation_action":
		return "prefer_action_then_refresh_results"
	case "review_preview", "review_session":
		if readCount > 1 {
			return "prefer_preview_then_session_then_queue"
		}
		return "prefer_primary_only"
	case "generation_queue":
		return "prefer_queue_then_session"
	default:
		return "prefer_primary_only"
	}
}

func NavigationDispatchMaxParallelism(strategy string) int {
	switch strategy {
	case "mutation_then_refresh":
		return 2
	case "fanout_read":
		return 3
	default:
		return 1
	}
}

func NavigationDispatchStepCachePreference(revalidateAfterAction bool, stepKind string) string {
	if revalidateAfterAction {
		return "revalidate"
	}
	switch stepKind {
	case "session", "preview":
		return "stale_while_revalidate"
	default:
		return "revalidate"
	}
}
