package platform

import (
	"errors"
	"testing"
)

func TestPlatformErrors(t *testing.T) {
	t.Parallel()

	if !errors.Is(ErrUnsupportedPlatform, ErrUnsupportedPlatform) {
		t.Fatal("ErrUnsupportedPlatform should be an error sentinel")
	}
	if ErrUnsupportedPlatform.Error() != "unsupported preview platform" {
		t.Fatalf("ErrUnsupportedPlatform = %q, want %q", ErrUnsupportedPlatform.Error(), "unsupported preview platform")
	}

	if !errors.Is(ErrPlatformUnavailable, ErrPlatformUnavailable) {
		t.Fatal("ErrPlatformUnavailable should be an error sentinel")
	}
	if ErrPlatformUnavailable.Error() != "preview platform unavailable" {
		t.Fatalf("ErrPlatformUnavailable = %q, want %q", ErrPlatformUnavailable.Error(), "preview platform unavailable")
	}
}
