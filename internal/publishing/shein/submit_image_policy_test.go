package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestCloneProductForSubmitDeepCopiesProduct(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		SupplierCode: "SUP-1",
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: "https://cdn.example.com/1.jpg"}},
		},
	}

	cloned, err := CloneProductForSubmit(product)
	if err != nil {
		t.Fatalf("CloneProductForSubmit() error = %v", err)
	}
	if cloned == nil || cloned.SupplierCode != product.SupplierCode {
		t.Fatalf("cloned product = %+v, want supplier code copied", cloned)
	}
	cloned.ImageInfo.ImageInfoList[0].ImageURL = "https://cdn.example.com/changed.jpg"
	if product.ImageInfo.ImageInfoList[0].ImageURL == cloned.ImageInfo.ImageInfoList[0].ImageURL {
		t.Fatal("CloneProductForSubmit() did not deep copy image list")
	}
}

func TestProductImageURLAndPendingUploadCounts(t *testing.T) {
	t.Parallel()

	product := &sheinproduct.Product{
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: "https://cdn.example.com/spu.jpg"},
				{ImageURL: "https://img.shein.com/uploaded.jpg"},
				{ImageURL: "  "},
			},
		},
		SKCList: []sheinproduct.SKC{{
			ImageInfo: sheinproduct.ImageInfo{
				ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: "https://cdn.example.com/skc.jpg"}},
			},
			SKUS: []sheinproduct.SKU{{
				ImageInfo: &sheinproduct.ImageInfo{
					ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: "https://ltwebstatic.com/sku.jpg"}},
				},
			}},
		}},
	}

	if got := ProductImageURLCount(product); got != 4 {
		t.Fatalf("ProductImageURLCount() = %d, want 4", got)
	}
	if got := ProductPendingImageUploadCount(product); got != 2 {
		t.Fatalf("ProductPendingImageUploadCount() = %d, want 2", got)
	}
}

func TestImageURLClassifiersAndCacheClone(t *testing.T) {
	t.Parallel()

	if !IsUploadedImageURL(" https://img.shein.com/uploaded.jpg ") {
		t.Fatal("IsUploadedImageURL(shein image) = false, want true")
	}
	if IsUploadedImageURL("https://cdn.example.com/source.jpg") {
		t.Fatal("IsUploadedImageURL(source image) = true, want false")
	}
	if !IsSDSImageURL("https://cdn.sdspod.com/source.jpg") {
		t.Fatal("IsSDSImageURL(sdspod) = false, want true")
	}

	cloned := CloneImageUploadCache(map[string]string{
		" https://cdn.example.com/source.jpg ": " https://img.shein.com/uploaded.jpg ",
		"https://cdn.example.com/bad.jpg":      "https://cdn.example.com/not-uploaded.jpg",
		"":                                     "https://img.shein.com/empty-key.jpg",
	})
	if len(cloned) != 1 || cloned["https://cdn.example.com/source.jpg"] != "https://img.shein.com/uploaded.jpg" {
		t.Fatalf("CloneImageUploadCache() = %#v, want only normalized uploaded cache entry", cloned)
	}
}
