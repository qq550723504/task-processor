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

func TestNormalizeSubmitGalleryImagesAddsSquareAndOptionalColorBlock(t *testing.T) {
	t.Parallel()

	images := []sheinproduct.ImageDetail{
		{ImageType: 1, ImageSort: 9, ImageURL: "https://img.example.com/main.jpg", MarketingMainImage: true, SizeImgFlag: true, TransformCVSizeImage: true},
		{ImageType: 2, ImageSort: 8, ImageURL: "https://img.example.com/detail.jpg"},
		{ImageType: 6, ImageSort: 7, ImageURL: "https://img.example.com/color.jpg"},
		{ImageType: 2, ImageSort: 6, ImageURL: "https://img.example.com/detail.jpg"},
	}

	normalized := NormalizeSubmitGalleryImages(images, true)

	if len(normalized) != 4 {
		t.Fatalf("normalized images len = %d, want 4", len(normalized))
	}
	wantTypes := []int{1, 2, 5, 6}
	for index, wantType := range wantTypes {
		if normalized[index].ImageType != wantType {
			t.Fatalf("image %d type = %d, want %d in %+v", index, normalized[index].ImageType, wantType, normalized)
		}
		if normalized[index].ImageSort != index+1 {
			t.Fatalf("image %d sort = %d, want %d", index, normalized[index].ImageSort, index+1)
		}
		if normalized[index].MarketingMainImage || normalized[index].SizeImgFlag || normalized[index].TransformCVSizeImage {
			t.Fatalf("image %d flags = %+v, want submit flags disabled", index, normalized[index])
		}
		if normalized[index].PSTypes == nil {
			t.Fatalf("image %d ps types nil, want empty slice", index)
		}
	}
	if normalized[3].ImageURL != "https://img.example.com/color.jpg" {
		t.Fatalf("color block URL = %q, want explicit color source", normalized[3].ImageURL)
	}
}
