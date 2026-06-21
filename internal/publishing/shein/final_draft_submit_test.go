package shein

import (
	"testing"
	"time"
)

func TestConfirmFinalSubmissionDraftInitializesAndMarksDraft(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 14, 0, 0, 0, time.UTC)
	pkg := &Package{}

	draft := ConfirmFinalSubmissionDraft(pkg, "publish", now)

	if draft == nil {
		t.Fatal("draft = nil, want initialized final draft")
	}
	if pkg.FinalSubmissionDraft != draft {
		t.Fatalf("FinalSubmissionDraft = %p, want returned draft %p", pkg.FinalSubmissionDraft, draft)
	}
	if !draft.Confirmed {
		t.Fatal("Confirmed = false, want true")
	}
	if draft.ConfirmedAt == nil || !draft.ConfirmedAt.Equal(now) {
		t.Fatalf("ConfirmedAt = %v, want %v", draft.ConfirmedAt, now)
	}
	if draft.UpdatedAt == nil || !draft.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", draft.UpdatedAt, now)
	}
	if draft.SubmitMode != "publish" {
		t.Fatalf("SubmitMode = %q, want publish", draft.SubmitMode)
	}
}

func TestConfirmFinalSubmissionDraftPreservesExistingSubmitMode(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 15, 0, 0, 0, time.UTC)
	pkg := &Package{FinalSubmissionDraft: &FinalDraft{SubmitMode: "save_draft"}}

	draft := ConfirmFinalSubmissionDraft(pkg, "publish", now)

	if draft.SubmitMode != "save_draft" {
		t.Fatalf("SubmitMode = %q, want existing save_draft", draft.SubmitMode)
	}
	if draft.ConfirmedAt == nil || !draft.ConfirmedAt.Equal(now) {
		t.Fatalf("ConfirmedAt = %v, want %v", draft.ConfirmedAt, now)
	}
}

func TestApplyFinalDraftUpdateNormalizesUserEditableFields(t *testing.T) {
	t.Parallel()

	confirmed := true
	order := []string{" https://cdn.example/b.jpg ", "https://cdn.example/a.jpg", "https://cdn.example/b.jpg", ""}
	deleted := []string{" https://cdn.example/deleted.jpg ", "", "https://cdn.example/deleted.jpg"}
	now := time.Date(2026, 6, 22, 9, 30, 0, 0, time.UTC)
	pkg := &Package{}

	draft := ApplyFinalDraftUpdate(pkg, FinalDraftUpdate{
		Confirmed:            &confirmed,
		SubmitMode:           " PUBLISH ",
		ManualPriceOverrides: map[string]float64{" SKU-1 ": 12.34, "": 99, "SKU-2": 0},
		FinalImageOrder:      &order,
		MainImageURL:         " https://cdn.example/main.jpg ",
		DeletedImageURLs:     &deleted,
		ImageRoleOverrides: map[string]string{
			" https://cdn.example/main.jpg ":  " MAIN ",
			"https://cdn.example/skc.jpg":     "swatch",
			"https://cdn.example/ignored.jpg": "banner",
		},
	}, now)

	if draft == nil {
		t.Fatal("draft = nil, want initialized final draft")
	}
	if draft.SubmitMode != "publish" {
		t.Fatalf("SubmitMode = %q, want publish", draft.SubmitMode)
	}
	if got := draft.ManualPriceOverrides[" SKU-1 "]; got != 12.34 {
		t.Fatalf("ManualPriceOverrides = %#v, want SKU-1 override", draft.ManualPriceOverrides)
	}
	if _, exists := draft.ManualPriceOverrides[""]; exists {
		t.Fatalf("ManualPriceOverrides kept empty SKU: %#v", draft.ManualPriceOverrides)
	}
	if len(draft.FinalImageOrder) != 2 || draft.FinalImageOrder[0] != "https://cdn.example/b.jpg" || draft.FinalImageOrder[1] != "https://cdn.example/a.jpg" {
		t.Fatalf("FinalImageOrder = %#v, want trimmed unique order", draft.FinalImageOrder)
	}
	if draft.MainImageURL != "https://cdn.example/main.jpg" {
		t.Fatalf("MainImageURL = %q, want trimmed main image", draft.MainImageURL)
	}
	if len(draft.DeletedImageURLs) != 1 || draft.DeletedImageURLs[0] != "https://cdn.example/deleted.jpg" {
		t.Fatalf("DeletedImageURLs = %#v, want trimmed unique deleted image", draft.DeletedImageURLs)
	}
	if draft.ImageRoleOverrides["https://cdn.example/main.jpg"] != "main" || draft.ImageRoleOverrides["https://cdn.example/skc.jpg"] != "swatch" {
		t.Fatalf("ImageRoleOverrides = %#v, want normalized accepted roles", draft.ImageRoleOverrides)
	}
	if _, exists := draft.ImageRoleOverrides["https://cdn.example/ignored.jpg"]; exists {
		t.Fatalf("ImageRoleOverrides kept unsupported role: %#v", draft.ImageRoleOverrides)
	}
	if !draft.Confirmed || draft.ConfirmedAt == nil || !draft.ConfirmedAt.Equal(now) {
		t.Fatalf("Confirmed/ConfirmedAt = %v/%v, want true/%v", draft.Confirmed, draft.ConfirmedAt, now)
	}
	if draft.UpdatedAt == nil || !draft.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", draft.UpdatedAt, now)
	}
}

func TestApplyFinalDraftUpdateIgnoresInvalidSubmitModeAndCanClearConfirmation(t *testing.T) {
	t.Parallel()

	confirmed := false
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	previousConfirmedAt := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	pkg := &Package{FinalSubmissionDraft: &FinalDraft{
		SubmitMode:  "save_draft",
		Confirmed:   true,
		ConfirmedAt: &previousConfirmedAt,
	}}

	draft := ApplyFinalDraftUpdate(pkg, FinalDraftUpdate{
		Confirmed:  &confirmed,
		SubmitMode: "delete",
	}, now)

	if draft.SubmitMode != "save_draft" {
		t.Fatalf("SubmitMode = %q, want preserved save_draft", draft.SubmitMode)
	}
	if draft.Confirmed {
		t.Fatal("Confirmed = true, want false")
	}
	if draft.ConfirmedAt != nil {
		t.Fatalf("ConfirmedAt = %v, want nil", draft.ConfirmedAt)
	}
	if draft.UpdatedAt == nil || !draft.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", draft.UpdatedAt, now)
	}
}
