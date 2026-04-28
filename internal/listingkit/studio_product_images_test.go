package listingkit

import "testing"

func TestStudioProductImageConcurrencyLimitMatchesImageCount(t *testing.T) {
	tests := []struct {
		name       string
		imageCount int
		want       int
	}{
		{name: "single image", imageCount: 1, want: 1},
		{name: "three images", imageCount: 3, want: 3},
		{name: "max images", imageCount: maxStudioProductImageCount, want: maxStudioProductImageCount},
		{name: "defensive empty count", imageCount: 0, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := studioProductImageConcurrencyLimit(tt.imageCount); got != tt.want {
				t.Fatalf("studioProductImageConcurrencyLimit(%d) = %d, want %d", tt.imageCount, got, tt.want)
			}
		})
	}
}
