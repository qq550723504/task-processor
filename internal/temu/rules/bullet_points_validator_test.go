package rules

import "testing"

func TestBulletPointsValidatorOptimizePointFormatUsesSubmissionFormatting(t *testing.T) {
	validator := &BulletPointsValidator{}

	got := validator.optimizePointFormat("widget(blue) , size . Next")
	want := "Widget (blue), size. Next"

	if got != want {
		t.Fatalf("optimizePointFormat() = %q, want %q", got, want)
	}
}

func TestBulletPointsValidatorOptimizePointFormatNormalizesUnicodeWhitespace(t *testing.T) {
	validator := &BulletPointsValidator{}

	got := validator.optimizePointFormat("widget\u00a0(blue)\u00a0,\u00a0size")
	want := "Widget (blue), size"

	if got != want {
		t.Fatalf("optimizePointFormat() = %q, want %q", got, want)
	}
}
