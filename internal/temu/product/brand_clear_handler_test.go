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
