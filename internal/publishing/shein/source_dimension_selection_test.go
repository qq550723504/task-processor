package shein

import "testing"

func TestSelectSourceDimensionsFallbackPrefersDescriptivePrimaryAndNumericSecondary(t *testing.T) {
	selection := selectSourceDimensionsFallback([]SourceVariantDimension{
		{Name: "颜色", Values: []string{"B-2601黑色", "B-2601黑灰色"}, DistinctCount: 2, SampleValue: "B-2601黑色"},
		{Name: "尺码", Values: []string{"39", "40", "41", "42", "43", "44"}, DistinctCount: 6, SampleValue: "39"},
	})

	if selection == nil {
		t.Fatalf("expected fallback selection")
	}
	if selection.PrimarySourceDimension != "颜色" {
		t.Fatalf("primary source dimension = %q, want 颜色", selection.PrimarySourceDimension)
	}
	if selection.SecondarySourceDimension != "尺码" {
		t.Fatalf("secondary source dimension = %q, want 尺码", selection.SecondarySourceDimension)
	}
}
