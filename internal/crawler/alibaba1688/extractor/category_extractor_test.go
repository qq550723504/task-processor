package extractor

import "testing"

func TestNormalizeCategoryPathFiltersBreadcrumbNoise(t *testing.T) {
	path := normalizeCategoryPath([]string{"首页", "家居饰品", "居家垫类", "坐垫/椅垫/沙发垫", "坐垫/椅垫/沙发垫"})
	want := "家居饰品 > 居家垫类 > 坐垫/椅垫/沙发垫"
	if path != want {
		t.Fatalf("normalizeCategoryPath() = %q, want %q", path, want)
	}
}

func TestSelectCategoryPathPrefersDeepestCandidate(t *testing.T) {
	raw := []any{
		[]any{"首页", "家居饰品"},
		[]any{"首页", "家居饰品", "居家垫类", "坐垫/椅垫/沙发垫"},
		[]any{"家居饰品", "居家垫类"},
	}

	got := selectCategoryPath(raw)
	want := "家居饰品 > 居家垫类 > 坐垫/椅垫/沙发垫"
	if got != want {
		t.Fatalf("selectCategoryPath() = %q, want %q", got, want)
	}
}
