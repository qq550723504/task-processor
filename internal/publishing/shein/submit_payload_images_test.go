package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestNormalizeSubmitImagesRepairsMissingSKUImageFromSKC(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{
					{ImageType: 1, ImageSort: 1, ImageURL: "https://img.example.com/skc-main.jpg"},
					{ImageType: 2, ImageSort: 2, ImageURL: "https://img.example.com/skc-gallery.jpg"},
				},
			},
			SKUS: []sheinproduct.SKU{{SupplierSKU: "SKU-1"}},
		}},
	}

	NormalizeSubmitImages(product)

	got := product.SKCList[0].SKUS[0].ImageInfo
	if got == nil || len(got.ImageInfoList) != 1 {
		t.Fatalf("sku image info = %+v, want single fallback image", got)
	}
	if got.ImageInfoList[0].ImageURL != "https://img.example.com/skc-main.jpg" {
		t.Fatalf("sku image URL = %q, want SKC main image", got.ImageInfoList[0].ImageURL)
	}
	if got.ImageInfoList[0].ImageType != 1 || got.ImageInfoList[0].ImageSort != 1 {
		t.Fatalf("sku image detail = %+v, want type=1 sort=1", got.ImageInfoList[0])
	}
	if got.OriginalImageInfoList == nil {
		t.Fatal("sku original image info list is nil, want empty list pointer")
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

func TestBuildSubmitSiteDetailImageInfoListPrefersDetailImages(t *testing.T) {
	t.Parallel()

	items := BuildSubmitSiteDetailImageInfoList([]sheinproduct.ImageDetail{
		{ImageType: 1, ImageURL: "https://img.example.com/main.jpg"},
		{ImageType: 2, ImageURL: "https://img.example.com/detail-1.jpg"},
		{ImageType: 2, ImageURL: "https://img.example.com/detail-2.jpg"},
		{ImageType: 5, ImageURL: "https://img.example.com/square.jpg"},
	})

	if len(items) != 1 || len(items[0].ImageInfoList) != 2 {
		t.Fatalf("site detail image info = %+v, want two detail images", items)
	}
	if items[0].ImageInfoList[0].ImageURL != "https://img.example.com/detail-1.jpg" || items[0].ImageInfoList[0].ImageSort != 1 {
		t.Fatalf("first detail image = %+v, want detail-1 sort 1", items[0].ImageInfoList[0])
	}
	if items[0].ImageInfoList[1].ImageURL != "https://img.example.com/detail-2.jpg" || items[0].ImageInfoList[1].ImageSort != 2 {
		t.Fatalf("second detail image = %+v, want detail-2 sort 2", items[0].ImageInfoList[1])
	}
}
