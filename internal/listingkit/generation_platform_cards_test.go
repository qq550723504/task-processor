package listingkit

import "testing"

func TestBuildSheinPreviewCardUsesWorkspaceSignals(t *testing.T) {
	t.Parallel()

	card, ok := buildSheinPreviewCard(&SheinPackage{
		Inspection:  &SheinInspection{NeedsReview: true, Summary: []string{"图像需复核", "类目待确认"}},
		ReviewNotes: []string{"manual review"},
	}, nil, nil)
	if !ok {
		t.Fatal("buildSheinPreviewCard() ok = false, want true")
	}
	if card.Status != "needs_review" {
		t.Fatalf("buildSheinPreviewCard().Status = %q, want %q", card.Status, "needs_review")
	}
	if card.Summary != "图像需复核；类目待确认" {
		t.Fatalf("buildSheinPreviewCard().Summary = %q, want inspection summary", card.Summary)
	}
	if !card.NeedsReview {
		t.Fatal("buildSheinPreviewCard().NeedsReview = false, want true")
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
