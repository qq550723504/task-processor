package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildPreviewCardSummaryPrefersInspectionSummary(t *testing.T) {
	t.Parallel()

	got := BuildPreviewCardSummary(&sheinpub.Package{
		SpuName:       "fallback name",
		ProductNameEn: "english name",
		Inspection:    &sheinpub.Inspection{Summary: []string{"图像需复核", "类目待确认"}},
	})
	if want := "图像需复核；类目待确认"; got != want {
		t.Fatalf("BuildPreviewCardSummary() = %q, want %q", got, want)
	}
}

func TestBuildPreviewCardStatusUsesInspectionNeedsReview(t *testing.T) {
	t.Parallel()

	if got := BuildPreviewCardStatus(&sheinpub.Package{
		Inspection: &sheinpub.Inspection{NeedsReview: true},
	}); got != "needs_review" {
		t.Fatalf("BuildPreviewCardStatus() = %q, want %q", got, "needs_review")
	}
}

func TestPreviewCardNeedsReviewFromReviewNotes(t *testing.T) {
	t.Parallel()

	if !PreviewCardNeedsReview(&sheinpub.Package{ReviewNotes: []string{"manual review"}}) {
		t.Fatal("expected review notes to mark SHEIN preview card as needs review")
	}
}
