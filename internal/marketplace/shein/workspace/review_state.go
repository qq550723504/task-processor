package workspace

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
)

const defaultInspectionReviewReason = "SHEIN 信息需要人工复核"

func InspectionReviewReasons(pkg *sheinpub.Package) []string {
	if pkg == nil || pkg.Inspection == nil || !pkg.Inspection.NeedsReview {
		return nil
	}

	reasons := normalizeReviewReasons(pkg.Inspection.Summary)
	if len(reasons) == 0 {
		reasons = normalizeReviewReasons(pkg.ReviewNotes)
	}
	if len(reasons) == 0 {
		return []string{defaultInspectionReviewReason}
	}
	return reasons
}

func CookieUnavailableReviewNotes(pkg *sheinpub.Package) []string {
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
		if IsCookieUnavailableText(note) {
			filtered = append(filtered, note)
		}
	}
	return filtered
}

func StripCookieUnavailableReviewNotes(pkg *sheinpub.Package) {
	if pkg == nil {
		return
	}
	pkg.ReviewNotes = FilterOutCookieUnavailableReviewNotes(pkg.ReviewNotes)
	if pkg.CategoryResolution != nil {
		pkg.CategoryResolution.ReviewNotes = FilterOutCookieUnavailableReviewNotes(pkg.CategoryResolution.ReviewNotes)
	}
	if pkg.AttributeResolution != nil {
		pkg.AttributeResolution.ReviewNotes = FilterOutCookieUnavailableReviewNotes(pkg.AttributeResolution.ReviewNotes)
	}
	if pkg.SaleAttributeResolution != nil {
		pkg.SaleAttributeResolution.ReviewNotes = FilterOutCookieUnavailableReviewNotes(pkg.SaleAttributeResolution.ReviewNotes)
	}
}

func FilterOutCookieUnavailableReviewNotes(notes []string) []string {
	if len(notes) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(notes))
	for _, note := range notes {
		if IsCookieUnavailableText(note) {
			continue
		}
		filtered = append(filtered, note)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func HasCookieUnavailableReviewNotes(pkg *sheinpub.Package) bool {
	return len(CookieUnavailableReviewNotes(pkg)) > 0
}

func IsCookieUnavailableText(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	return strings.Contains(text, "cookie 不可用") ||
		strings.Contains(text, "cookies are unavailable") ||
		strings.Contains(text, "store cookies are unavailable") ||
		strings.Contains(text, "店铺 cookie")
}

func normalizeReviewReasons(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	reasons := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
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
