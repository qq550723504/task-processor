package listingkit

import "strings"

func normalizeReviewReasons(values []string) []string {
	return uniqueStrings(values)
}

func reviewReasonsFromResult(result *ListingKitResult) []string {
	if result == nil {
		return nil
	}
	sources := reviewReasonSources{
		WorkflowReasons: workflowIssueMessagesBySeverity(
			result,
			WorkflowIssueSeverityReview,
			WorkflowIssueSeverityBlocking,
		),
		ResultReasons:   result.ReviewReasons,
		PlatformReasons: reviewReasonsFromPlatformPackages(result),
	}
	if result.Summary != nil {
		sources.SummaryNeedsReview = result.Summary.NeedsReview
		sources.SummaryWarnings = result.Summary.Warnings
	}
	if result.PodExecution != nil {
		sources.PodBlocked = result.PodExecution.Status == podStatusFailedBlocking
		sources.PodFailureReason = result.PodExecution.FailureReason
	}
	return resolveReviewReasons(sources)
}

func reviewReasonsFromTask(task *Task) []string {
	if task == nil {
		return nil
	}
	if reasons := reviewReasonsFromResult(task.Result); len(reasons) > 0 {
		return reasons
	}
	if value := strings.TrimSpace(task.Error); value != "" {
		return []string{value}
	}
	return nil
}

func reviewReasonsFromPlatformPackages(result *ListingKitResult) []string {
	if result == nil {
		return nil
	}
	reasons := make([]string, 0, 8)
	if result.Shein != nil {
		if result.Shein.Inspection != nil && result.Shein.Inspection.NeedsReview {
			reasons = append(reasons, result.Shein.Inspection.Summary...)
		}
		reasons = append(reasons, result.Shein.ReviewNotes...)
	}
	if result.Temu != nil {
		reasons = append(reasons, result.Temu.ReviewNotes...)
	}
	if result.Walmart != nil {
		reasons = append(reasons, result.Walmart.ReviewNotes...)
	}
	return normalizeReviewReasons(reasons)
}
