package studio

import "testing"

func TestBuildPrintableHintFormatsRequestedPrintArea(t *testing.T) {
	got := BuildPrintableHint(1000, 600)
	want := "Mandatory print size requirement: target print area: 1000 by 600 pixels. Preserve this exact 1000:600 aspect ratio. Compose the artwork for this requested print area and do not output a square design unless the requested print area is square."
	if got != want {
		t.Fatalf("BuildPrintableHint() = %q, want %q", got, want)
	}
}

func TestBuildPrintableHintReturnsEmptyForNonPositiveDimensions(t *testing.T) {
	if got := BuildPrintableHint(1000, 0); got != "" {
		t.Fatalf("BuildPrintableHint() = %q, want empty", got)
	}
}
