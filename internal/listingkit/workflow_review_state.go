package listingkit

import "strings"

const (
	sheinCookieUnavailableIssueCode = "shein_cookie_unavailable"
	sheinCookieUnavailableMessage   = "SHEIN 店铺 cookie 不可用，需要重新登录店铺后重试在线解析"
)

func applySheinInspectionReviewToSummary(result *ListingKitResult) {
	if result == nil || result.Shein == nil || result.Shein.Inspection == nil || !result.Shein.Inspection.NeedsReview {
		return
	}
	if result.Summary == nil {
		result.Summary = &GenerationSummary{}
	}
	result.Summary.NeedsReview = true

	reasons := normalizeReviewReasons(result.Shein.Inspection.Summary)
	if len(reasons) == 0 {
		reasons = normalizeReviewReasons(result.Shein.ReviewNotes)
	}
	if len(reasons) == 0 {
		reasons = []string{"SHEIN 信息需要人工复核"}
	}
	result.Summary.Warnings = uniqueStrings(append(result.Summary.Warnings, reasons...))
	result.ReviewReasons = uniqueStrings(append(result.ReviewReasons, reasons...))
}

func addSheinReviewWorkflowIssues(result *ListingKitResult) {
	if result == nil {
		return
	}
	recorder := newWorkflowRecorder(result)
	cookieNotes := sheinCookieUnavailableReviewNotes(result.Shein)
	for _, note := range cookieNotes {
		recorder.AddIssue(
			WorkflowIssueSeverityBlocking,
			"shein_review",
			sheinCookieUnavailableIssueCode,
			sheinCookieUnavailableMessage,
			note,
		)
	}
	if result.Summary == nil || !result.Summary.NeedsReview {
		return
	}
	for _, reason := range reviewReasonsFromResult(result) {
		if isSheinCookieUnavailableText(reason) {
			continue
		}
		recorder.AddIssue(WorkflowIssueSeverityReview, "shein_review", "shein_review_required", reason, "")
	}
}

func sheinCookieUnavailableReviewNotes(pkg *SheinPackage) []string {
	if pkg == nil {
		return nil
	}
	notes := make([]string, 0, 4)
	notes = append(notes, pkg.ReviewNotes...)
	if pkg.CategoryResolution != nil {
		notes = append(notes, pkg.CategoryResolution.ReviewNotes...)
	}
	if pkg.AttributeResolution != nil {
		notes = append(notes, pkg.AttributeResolution.ReviewNotes...)
	}
	if pkg.SaleAttributeResolution != nil {
		notes = append(notes, pkg.SaleAttributeResolution.ReviewNotes...)
	}
	filtered := make([]string, 0, len(notes))
	for _, note := range normalizeReviewReasons(notes) {
		if isSheinCookieUnavailableText(note) {
			filtered = append(filtered, note)
		}
	}
	return filtered
}

func stripSheinCookieUnavailableReviewNotes(pkg *SheinPackage) {
	if pkg == nil {
		return
	}
	pkg.ReviewNotes = filterOutSheinCookieUnavailableReviewNotes(pkg.ReviewNotes)
	if pkg.CategoryResolution != nil {
		pkg.CategoryResolution.ReviewNotes = filterOutSheinCookieUnavailableReviewNotes(pkg.CategoryResolution.ReviewNotes)
	}
	if pkg.AttributeResolution != nil {
		pkg.AttributeResolution.ReviewNotes = filterOutSheinCookieUnavailableReviewNotes(pkg.AttributeResolution.ReviewNotes)
	}
	if pkg.SaleAttributeResolution != nil {
		pkg.SaleAttributeResolution.ReviewNotes = filterOutSheinCookieUnavailableReviewNotes(pkg.SaleAttributeResolution.ReviewNotes)
	}
}

func filterOutSheinCookieUnavailableReviewNotes(notes []string) []string {
	if len(notes) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(notes))
	for _, note := range notes {
		if isSheinCookieUnavailableText(note) {
			continue
		}
		filtered = append(filtered, note)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func sheinCookieUnavailable(pkg *SheinPackage) bool {
	return len(sheinCookieUnavailableReviewNotes(pkg)) > 0
}

func isSheinCookieUnavailableText(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	return strings.Contains(text, "cookie 不可用") ||
		strings.Contains(text, "cookies are unavailable") ||
		strings.Contains(text, "store cookies are unavailable") ||
		strings.Contains(text, "店铺 cookie")
}
