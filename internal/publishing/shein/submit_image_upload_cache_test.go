package shein

import (
	"testing"
	"time"
)

func TestApplyImageUploadCacheStoresCacheOnFinalSubmissionDraft(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 21, 18, 30, 0, 0, time.UTC)
	cache := map[string]string{"source": "https://img.shein.com/uploaded.jpg"}
	pkg := &Package{}

	if !ApplyImageUploadCache(pkg, cache, now) {
		t.Fatal("ApplyImageUploadCache() = false, want true")
	}
	if pkg.FinalSubmissionDraft == nil {
		t.Fatal("FinalSubmissionDraft = nil, want initialized")
	}
	if pkg.FinalSubmissionDraft.SheinImageUploadCache["source"] != cache["source"] {
		t.Fatalf("cache = %#v, want stored upload cache", pkg.FinalSubmissionDraft.SheinImageUploadCache)
	}
	if pkg.FinalSubmissionDraft.UpdatedAt == nil || !pkg.FinalSubmissionDraft.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", pkg.FinalSubmissionDraft.UpdatedAt, now)
	}
}

func TestApplyImageUploadCacheSkipsEmptyCache(t *testing.T) {
	t.Parallel()

	pkg := &Package{}

	if ApplyImageUploadCache(pkg, nil, time.Now()) {
		t.Fatal("ApplyImageUploadCache(empty) = true, want false")
	}
	if pkg.FinalSubmissionDraft != nil {
		t.Fatalf("FinalSubmissionDraft = %+v, want nil", pkg.FinalSubmissionDraft)
	}
}
