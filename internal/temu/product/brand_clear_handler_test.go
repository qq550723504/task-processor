package product

import "testing"

func TestBrandClearRemoveBrandFromTextNormalizesSubmissionFormatting(t *testing.T) {
	handler := &BrandClearHandler{}

	got := handler.removeBrandFromText("Acme Widget(Blue) , Size . Next", "Acme")
	want := "Widget (Blue), Size. Next"

	if got != want {
		t.Fatalf("removeBrandFromText() = %q, want %q", got, want)
	}
}

func TestBrandClearRemoveBrandFromTextPreservesUnicodeWhitespaceNormalization(t *testing.T) {
	handler := &BrandClearHandler{}

	got := handler.removeBrandFromText("Acme\u00a0Widget\u00a0(Blue)\u00a0,\u00a0Size", "Acme")
	want := "Widget (Blue), Size"

	if got != want {
		t.Fatalf("removeBrandFromText() = %q, want %q", got, want)
	}
}
