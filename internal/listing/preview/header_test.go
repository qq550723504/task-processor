package preview

import (
	"slices"
	"testing"
)

func TestBuildHeader(t *testing.T) {
	t.Parallel()

	header := BuildHeader(HeaderInput{
		Country:       "US",
		Language:      "en",
		SourceType:    "amazon",
		ImageCount:    3,
		VariantCount:  2,
		StatusMessage: "预览结果已生成",
		Warnings:      []string{"warn-1", "warn-2"},
		ReviewReasons: []string{"review-1"},
		PlatformCards: []PlatformCard{{Platform: "amazon", Status: "ready"}},
	})
	if header == nil {
		t.Fatal("header = nil")
	}
	if header.Country != "US" || header.SourceType != "amazon" {
		t.Fatalf("header = %+v", header)
	}
	if !slices.Equal(header.Warnings, []string{"warn-1", "warn-2"}) {
		t.Fatalf("warnings = %#v", header.Warnings)
	}
	if !slices.Equal(header.ReviewReasons, []string{"review-1"}) {
		t.Fatalf("review reasons = %#v", header.ReviewReasons)
	}
	if len(header.PlatformCards) != 1 || header.PlatformCards[0].Platform != "amazon" {
		t.Fatalf("platform cards = %#v", header.PlatformCards)
	}
}
