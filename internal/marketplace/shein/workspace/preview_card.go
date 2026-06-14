package workspace

import sheinpub "task-processor/internal/publishing/shein"

func BuildPreviewCardStatus(pkg *sheinpub.Package) string {
	if pkg != nil && pkg.Inspection != nil && pkg.Inspection.NeedsReview {
		return "needs_review"
	}
	return "ready"
}

func BuildPreviewCardSummary(pkg *sheinpub.Package) string {
	summary := "已生成 SHEIN 预览"
	if pkg != nil {
		summary = firstNonEmpty(pkg.SpuName, pkg.ProductNameEn, summary)
	}
	if pkg != nil && pkg.Inspection != nil {
		summary = firstNonEmpty(joinStrings(pkg.Inspection.Summary, "；"), summary)
	}
	return summary
}

func PreviewCardNeedsReview(pkg *sheinpub.Package) bool {
	if pkg == nil {
		return false
	}
	if len(pkg.ReviewNotes) > 0 {
		return true
	}
	return pkg.Inspection != nil && pkg.Inspection.NeedsReview
}
