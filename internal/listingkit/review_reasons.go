package listingkit

import "strings"

func normalizeReviewReasons(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	reasons := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		reasons = append(reasons, value)
	}
	if len(reasons) == 0 {
		return nil
	}
	return reasons
}

func reviewReasonsFromResult(result *ListingKitResult) []string {
	if result == nil {
		return nil
	}
	if reasons := workflowIssueMessagesBySeverity(result, WorkflowIssueSeverityReview, WorkflowIssueSeverityBlocking); len(reasons) > 0 {
		return reasons
	}
	if reasons := normalizeReviewReasons(result.ReviewReasons); len(reasons) > 0 {
		return reasons
	}
	if result.Summary != nil && result.Summary.NeedsReview {
		if reasons := normalizeReviewReasons(result.Summary.Warnings); len(reasons) > 0 {
			return reasons
		}
	}
	return reviewReasonsFromPlatformPackages(result)
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
