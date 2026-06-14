package workspace

import (
	"slices"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildPreviewReviewSummary(t *testing.T) {
	t.Parallel()

	needsReview, summary := BuildPreviewReviewSummary(&sheinpub.Package{
		ReviewNotes: []string{"缺少类目", "缺少类目"},
		Inspection: &sheinpub.Inspection{
			NeedsReview: true,
			Summary:     []string{"图片待确认", "缺少类目"},
		},
	})
	if !needsReview {
		t.Fatal("needsReview = false, want true")
	}
	want := []string{"缺少类目", "图片待确认"}
	if !slices.Equal(summary, want) {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
}

func TestBuildFinalReviewSKU(t *testing.T) {
	t.Parallel()

	sku := sheinpub.SKUDraft{
		SupplierSKU: "SKU-1",
		BasePrice:   "12.50",
		Currency:    "USD",
		StockCount:  8,
		Weight:      0.3,
		SaleAttributes: []sheinpub.ResolvedSaleAttribute{
			{Name: "颜色", Value: "Black"},
			{Name: "尺码", Value: "One Size"},
		},
	}

	item := BuildFinalReviewSKU("SKC-1", sku)
	if item.SupplierCode != "SKC-1" || item.SupplierSKU != "SKU-1" {
		t.Fatalf("item = %+v", item)
	}
	if item.Color != "Black" || item.Size != "One Size" {
		t.Fatalf("item attrs = %+v", item)
	}
}

func TestBuildFinalReviewImagesDeduplicatesAndMarksSizeMap(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/main.jpg"
	sizeMapImage := "https://cdn.example.com/size.jpg"
	draft := &sheinpub.RequestDraft{
		ImageInfo: &sheinpub.ImageDraft{
			MainImage: mainImage,
			Gallery:   []string{mainImage, sizeMapImage},
		},
	}
	product := &sheinproduct.Product{
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: sizeMapImage, SizeImgFlag: true},
			},
		},
	}

	images := BuildFinalReviewImages(draft, &sheinpub.FinalDraft{MainImageURL: mainImage}, product)

	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2 (%+v)", len(images), images)
	}
	if images[0].Role != "main" || !images[0].Main {
		t.Fatalf("images[0] = %+v, want main image", images[0])
	}
	if images[1].Role != "size_map" || !images[1].SizeMap {
		t.Fatalf("images[1] = %+v, want size_map image", images[1])
	}
}
