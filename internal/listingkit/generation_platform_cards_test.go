package listingkit

import "testing"

func TestBuildSheinPreviewCardSummaryPrefersInspectionSummary(t *testing.T) {
	t.Parallel()

	got := buildSheinPreviewCardSummary(&SheinPackage{
		SpuName:       "fallback name",
		ProductNameEn: "english name",
		Inspection:    &SheinInspection{Summary: []string{"图像需复核", "类目待确认"}},
	})
	if want := "图像需复核；类目待确认"; got != want {
		t.Fatalf("buildSheinPreviewCardSummary() = %q, want %q", got, want)
	}
}

func TestBuildSheinPreviewCardStatusUsesInspectionNeedsReview(t *testing.T) {
	t.Parallel()

	if got := buildSheinPreviewCardStatus(&SheinPackage{
		Inspection: &SheinInspection{NeedsReview: true},
	}); got != "needs_review" {
		t.Fatalf("buildSheinPreviewCardStatus() = %q, want %q", got, "needs_review")
	}
}

func TestSheinPreviewCardNeedsReviewFromReviewNotes(t *testing.T) {
	t.Parallel()

	if !sheinPreviewCardNeedsReview(&SheinPackage{ReviewNotes: []string{"manual review"}}) {
		t.Fatal("expected review notes to mark SHEIN preview card as needs review")
	}
}

func TestBuildReviewNotePreviewCard(t *testing.T) {
	t.Parallel()

	card := buildReviewNotePreviewCard("temu", "fallback", true, "summary")
	if card.Status != "needs_review" {
		t.Fatalf("buildReviewNotePreviewCard().Status = %q, want %q", card.Status, "needs_review")
	}
	if card.Summary != "summary" {
		t.Fatalf("buildReviewNotePreviewCard().Summary = %q, want %q", card.Summary, "summary")
	}
	if !card.NeedsReview {
		t.Fatal("buildReviewNotePreviewCard().NeedsReview = false, want true")
	}
}
