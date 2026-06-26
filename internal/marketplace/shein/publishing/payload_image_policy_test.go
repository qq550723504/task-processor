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

func TestNormalizeSubmitSKUImageDetailResetsSubmitFields(t *testing.T) {
	t.Parallel()

	image := NormalizeSubmitSKUImageDetail(sheinproduct.ImageDetail{
		ImageURL:             "https://img.example.com/sku.jpg",
		ImageType:            6,
		ImageSort:            9,
		MarketingMainImage:   true,
		SizeImgFlag:          true,
		TransformCVSizeImage: true,
	})

	if image.ImageType != 1 || image.ImageSort != 1 {
		t.Fatalf("image type/sort = %d/%d, want 1/1", image.ImageType, image.ImageSort)
	}
	if image.MarketingMainImage || image.SizeImgFlag || image.TransformCVSizeImage {
		t.Fatalf("submit flags = %+v, want disabled", image)
	}
	if image.PSTypes == nil {
		t.Fatal("PSTypes nil, want empty slice")
	}
}
