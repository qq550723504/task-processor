package publishing

import "testing"

func TestComparableAttributeSegmentsSplitsCompoundValues(t *testing.T) {
	t.Parallel()

	got := ComparableAttributeSegments("red / blue， green;white")
	want := []string{"red", "blue", "green", "white"}
	if len(got) != len(want) {
		t.Fatalf("ComparableAttributeSegments() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ComparableAttributeSegments()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestComparableAttributeSegmentsIgnoresSingleOrEmptySegments(t *testing.T) {
	t.Parallel()

	if got := ComparableAttributeSegments("red"); got != nil {
		t.Fatalf("single segment = %#v, want nil", got)
	}
	got := ComparableAttributeSegments("red// blue")
	want := []string{"red", "blue"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("empty segments = %#v, want %#v", got, want)
	}
}
