package strx

import "testing"

func TestJoinNonBlankTrimsAndDropsBlankValues(t *testing.T) {
	t.Parallel()

	got := JoinNonBlank([]string{" alpha ", "", "  ", "beta"}, " > ")
	if got != "alpha > beta" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinNonBlankKeepsDuplicateNonBlankValues(t *testing.T) {
	t.Parallel()

	got := JoinNonBlank([]string{"x", "x"}, ",")
	if got != "x,x" {
		t.Fatalf("got %q", got)
	}
}
