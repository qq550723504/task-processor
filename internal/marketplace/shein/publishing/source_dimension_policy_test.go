package publishing

import "testing"

func TestSelectSourceDimensionsFallbackPrefersDescriptivePrimaryAndNumericSecondary(t *testing.T) {
	t.Parallel()

	selection := SelectSourceDimensionsFallback([]SourceDimension{
		{Name: "颜色", Values: []string{"B-2601黑色", "B-2601黑灰色"}, DistinctCount: 2},
		{Name: "尺码", Values: []string{"39", "40", "41", "42", "43", "44"}, DistinctCount: 6},
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

func TestSourceDimensionExistsNormalizesNames(t *testing.T) {
	t.Parallel()

	dimensions := []SourceDimension{{Name: "Style-Type"}}
	if !SourceDimensionExists(dimensions, "style type") {
		t.Fatalf("expected normalized name match")
	}
}

func TestIsNumericLikeSourceDimensionValueHandlesScaleAndCodePrefixes(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"EU 39", "10/12"} {
		if !IsNumericLikeSourceDimensionValue(value) {
			t.Fatalf("value %q should be numeric-like", value)
		}
	}
	for _, value := range []string{"blue", "B-2601黑色"} {
		if IsNumericLikeSourceDimensionValue(value) {
			t.Fatalf("value %q should not be numeric-like", value)
		}
	}
}
