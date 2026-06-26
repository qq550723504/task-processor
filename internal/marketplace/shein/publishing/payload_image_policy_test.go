package publishing

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestDedupeImagesByURLKeepsFirstNonEmptyURL(t *testing.T) {
	t.Parallel()

	images := []sheinproduct.ImageDetail{
		{ImageURL: " https://img.example.com/main.jpg ", ImageType: 1},
		{ImageURL: "https://img.example.com/detail.jpg", ImageType: 2},
		{ImageURL: "https://img.example.com/detail.jpg", ImageType: 6},
		{ImageURL: "  ", ImageType: 5},
	}

	got := DedupeImagesByURL(images)
	if len(got) != 2 {
		t.Fatalf("DedupeImagesByURL() len = %d, want 2", len(got))
	}
	if got[0].ImageURL != " https://img.example.com/main.jpg " || got[0].ImageType != 1 {
		t.Fatalf("first image = %+v, want original first image preserved", got[0])
	}
	if got[1].ImageURL != "https://img.example.com/detail.jpg" || got[1].ImageType != 2 {
		t.Fatalf("second image = %+v, want first detail image preserved", got[1])
	}
}
