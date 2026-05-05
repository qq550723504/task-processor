package listingkit

import (
	"strings"
	"testing"
)

func TestApplySheinInspectionReviewToSummaryMarksResultNeedsReview(t *testing.T) {
	result := &ListingKitResult{
		Summary: &GenerationSummary{},
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{"建议复核 SHEIN 类目"},
			},
		},
	}

	applySheinInspectionReviewToSummary(result)

	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs_review", result.Summary)
	}
	if got := strings.Join(result.Summary.Warnings, "\n"); !strings.Contains(got, "建议复核 SHEIN 类目") {
		t.Fatalf("summary warnings = %#v, want SHEIN inspection summary", result.Summary.Warnings)
	}
	if got := strings.Join(result.ReviewReasons, "\n"); !strings.Contains(got, "建议复核 SHEIN 类目") {
		t.Fatalf("review reasons = %#v, want SHEIN inspection summary", result.ReviewReasons)
	}
}
