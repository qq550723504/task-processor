package namelimit

import (
	"testing"

	"task-processor/internal/shein/api/product"
)

func TestNormalize(t *testing.T) {
	t.Parallel()

	limits := Normalize([]product.NameLengthConfigItem{
		{Language: " ZH-CN ", MaxLength: 100},
		{Language: "", MaxLength: 9},
		{Language: "en", MaxLength: 0},
	})
	if max, ok := limits.Max("zh-CN"); !ok || max != 100 {
		t.Fatalf("Max(zh-CN) = %d, %v; want 100, true", max, ok)
	}
	if _, ok := limits.Max("en"); ok {
		t.Fatal("Max(en) found invalid non-positive limit")
	}
}

func TestTruncateCountsUnicodeCharacters(t *testing.T) {
	t.Parallel()

	if got := Truncate("一二三四五", 3); got != "一二三" {
		t.Fatalf("Truncate() = %q, want %q", got, "一二三")
	}
}

func TestTruncatePrefersNearbyWordBoundary(t *testing.T) {
	t.Parallel()

	if got := Truncate("alpha beta gamma", 12); got != "alpha beta" {
		t.Fatalf("Truncate() = %q, want %q", got, "alpha beta")
	}
}
