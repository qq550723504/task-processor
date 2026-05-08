package generation

import "testing"

func TestBuildConditionalState(t *testing.T) {
	t.Parallel()

	if got := BuildConditionalState(" ", false, false); got != nil {
		t.Fatalf("BuildConditionalState(blank,false,false) = %+v, want nil", got)
	}

	got := BuildConditionalState(" token-1 ", true, false)
	if got == nil {
		t.Fatalf("BuildConditionalState() = nil, want state")
	}
	if got.DeltaToken != "token-1" || got.ETag != `"token-1"` || !got.NotModified || got.NoChanges {
		t.Fatalf("BuildConditionalState() = %+v, want trimmed token with ETag and not-modified", got)
	}
}

func TestConditionalETag(t *testing.T) {
	t.Parallel()

	if got := ConditionalETag(" token-1 "); got != `"token-1"` {
		t.Fatalf("ConditionalETag() = %q, want quoted token", got)
	}
	if got := ConditionalETag(" "); got != "" {
		t.Fatalf("ConditionalETag(blank) = %q, want empty", got)
	}
}

func TestIsReadNotModified(t *testing.T) {
	t.Parallel()

	if !IsReadNotModified(" token-1 ", "", "token-1") {
		t.Fatalf("IsReadNotModified() should match delta token")
	}
	if !IsReadNotModified("", " token-1 ", "token-1") {
		t.Fatalf("IsReadNotModified() should match If-Match")
	}
	if IsReadNotModified("", "", "token-1") {
		t.Fatalf("IsReadNotModified() should reject missing query tokens")
	}
	if IsReadNotModified("token-1", "", " ") {
		t.Fatalf("IsReadNotModified() should reject blank current token")
	}
}
