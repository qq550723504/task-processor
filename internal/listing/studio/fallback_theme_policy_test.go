package studio

import (
	"slices"
	"testing"
)

func TestBuildFallbackDesignThemesTrimsPromptAndRepeatsRequestedCount(t *testing.T) {
	got := BuildFallbackDesignThemes("  retro dog badge  ", 3)

	want := []string{"retro dog badge", "retro dog badge", "retro dog badge"}
	if !slices.Equal(got, want) {
		t.Fatalf("BuildFallbackDesignThemes() = %#v, want %#v", got, want)
	}
}

func TestBuildFallbackDesignThemesReturnsNilForNonPositiveCount(t *testing.T) {
	if got := BuildFallbackDesignThemes("retro dog badge", 0); got != nil {
		t.Fatalf("BuildFallbackDesignThemes() = %#v, want nil", got)
	}
}
