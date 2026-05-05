package listingkit

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
