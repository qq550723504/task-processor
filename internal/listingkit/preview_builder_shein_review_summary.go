package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func buildSheinPreviewReviewSummary(pkg *sheinpub.Package) (bool, []string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false, nil
	}
	needsReview := len(pkg.ReviewNotes) > 0
	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	if pkg.Inspection != nil {
		needsReview = needsReview || pkg.Inspection.NeedsReview
		summary = uniqueStrings(append(summary, pkg.Inspection.Summary...))
	}
	return needsReview, summary
}
