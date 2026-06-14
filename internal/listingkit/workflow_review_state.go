package listingkit

import (
	"strings"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
)

const (
	sheinCookieUnavailableIssueCode = "shein_cookie_unavailable"
	sheinCookieUnavailableMessage   = "SHEIN 店铺 cookie 不可用，需要重新登录店铺后重试在线解析"
)

func applySheinInspectionReviewToSummary(result *ListingKitResult) {
	reasons := sheinInspectionReviewReasons(result)
	if len(reasons) == 0 {
		return
	}
	if result.Summary == nil {
		result.Summary = &GenerationSummary{}
	}
	result.Summary.NeedsReview = true
	result.Summary.Warnings = uniqueStrings(append(result.Summary.Warnings, reasons...))
	result.ReviewReasons = uniqueStrings(append(result.ReviewReasons, reasons...))
}

func applySheinVariantCoverageReviewToSummary(result *ListingKitResult) {
	coverageWarning, blocked := sheinVariantImageCoverageStatus(result.Shein)
	coverageWarning = strings.TrimSpace(coverageWarning)
	if !blocked || coverageWarning == "" {
		return
	}
	if result.Summary == nil {
		result.Summary = &GenerationSummary{}
	}
	if result.Shein != nil {
		result.Shein.ReviewNotes = uniqueStrings(append(result.Shein.ReviewNotes, coverageWarning))
	}
	result.Summary.NeedsReview = true
	result.Summary.Warnings = uniqueStrings(append(result.Summary.Warnings, coverageWarning))
	result.ReviewReasons = uniqueStrings(append(result.ReviewReasons, coverageWarning))
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
	for _, reason := range sheinReviewIssueReasons(result) {
		recorder.AddIssue(WorkflowIssueSeverityReview, "shein_review", "shein_review_required", reason, "")
	}
}

func sheinInspectionReviewReasons(result *ListingKitResult) []string {
	if result == nil {
		return nil
	}
	return sheinworkspace.InspectionReviewReasons(result.Shein)
}

func sheinReviewIssueReasons(result *ListingKitResult) []string {
	coverageWarning, coverageBlocked := sheinVariantImageCoverageStatus(result.Shein)
	coverageWarning = strings.TrimSpace(coverageWarning)

	filtered := make([]string, 0)
	for _, reason := range sheinInspectionReviewReasons(result) {
		if sheinworkspace.IsCookieUnavailableText(reason) {
			continue
		}
		if coverageBlocked && coverageWarning != "" && strings.TrimSpace(reason) == coverageWarning {
			continue
		}
		filtered = append(filtered, reason)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func sheinCookieUnavailableReviewNotes(pkg *SheinPackage) []string {
	return sheinworkspace.CookieUnavailableReviewNotes(pkg)
}

func stripSheinCookieUnavailableReviewNotes(pkg *SheinPackage) {
	sheinworkspace.StripCookieUnavailableReviewNotes(pkg)
}

func filterOutSheinCookieUnavailableReviewNotes(notes []string) []string {
	return sheinworkspace.FilterOutCookieUnavailableReviewNotes(notes)
}

func sheinCookieUnavailable(pkg *SheinPackage) bool {
	return sheinworkspace.HasCookieUnavailableReviewNotes(pkg)
}

func isSheinCookieUnavailableText(value string) bool {
	return sheinworkspace.IsCookieUnavailableText(value)
}
