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

	images := BuildFinalReviewImages(draft, &sheinpub.FinalDraft{MainImageURL: mainImage}, product, nil)

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

func TestBuildFinalReviewImagesIncludesPreviewSKCDetailImages(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/main.jpg"
	firstDetail := "https://cdn.example.com/detail-1.jpg"
	secondSKCMain := "https://cdn.example.com/second-skc-main.jpg"
	secondDetail := "https://cdn.example.com/second-detail.jpg"
	draft := &sheinpub.RequestDraft{
		ImageInfo: &sheinpub.ImageDraft{
			MainImage: mainImage,
			Gallery:   []string{firstDetail},
		},
		SKCList: []sheinpub.SKCRequestDraft{
			{ImageInfo: &sheinpub.ImageDraft{MainImage: mainImage}},
			{ImageInfo: &sheinpub.ImageDraft{MainImage: secondSKCMain}},
		},
	}
	product := &sheinproduct.Product{
		ImageInfo: &sheinproduct.ImageInfo{
			ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: mainImage, ImageType: 1, ImageSort: 1, MarketingMainImage: true},
				{ImageURL: firstDetail, ImageType: 2, ImageSort: 2},
			},
		},
		SKCList: []sheinproduct.SKC{
			{
				ImageInfo: sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{
					{ImageURL: mainImage, ImageType: 1, ImageSort: 1, MarketingMainImage: true},
					{ImageURL: firstDetail, ImageType: 2, ImageSort: 2},
				}},
			},
			{
				ImageInfo: sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{
					{ImageURL: secondSKCMain, ImageType: 1, ImageSort: 1, MarketingMainImage: true},
					{ImageURL: secondDetail, ImageType: 2, ImageSort: 2},
				}},
			},
		},
	}

	images := BuildFinalReviewImages(draft, nil, product, nil)

	if len(images) != 4 {
		t.Fatalf("len(images) = %d, want 4 (%+v)", len(images), images)
	}
	if images[2].URL != secondSKCMain || images[2].Role != "skc" {
		t.Fatalf("second skc main = %+v, want skc role", images[2])
	}
	if images[3].URL != secondDetail || images[3].Role != "gallery" {
		t.Fatalf("second skc detail = %+v, want gallery role", images[3])
	}
}

func TestBuildFinalReviewImagesKeepsUnselectedSourceImagesAvailable(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/generated-main.jpg"
	sourceOne := "https://1688.example.com/source-1.jpg"
	sourceTwo := "https://1688.example.com/source-2.jpg"
	draft := &sheinpub.RequestDraft{
		ImageInfo: &sheinpub.ImageDraft{
			MainImage: mainImage,
			Source:    []string{sourceOne, sourceTwo},
		},
	}

	images := BuildFinalReviewImages(draft, nil, nil, nil)

	if len(images) != 3 {
		t.Fatalf("len(images) = %d, want 3 (%+v)", len(images), images)
	}
	if !images[0].Final || !images[0].Selected || images[0].Origin != "generated" {
		t.Fatalf("generated image = %+v, want selected generated image", images[0])
	}
	for index, want := range []string{sourceOne, sourceTwo} {
		image := images[index+1]
		if image.URL != want || image.Final || image.Selected || image.Origin != "source" || !image.RequiresReview {
			t.Fatalf("source image %d = %+v, want unselected source image", index, image)
		}
	}
}

func TestBuildFinalReviewImagesPreservesOfferedSourceImageProvenance(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/generated-main.jpg"
	sourceImage := "https://1688.example.com/offered-source.jpg"
	draft := &sheinpub.RequestDraft{
		ImageInfo: &sheinpub.ImageDraft{
			MainImage: mainImage,
			Gallery:   []string{sourceImage},
		},
	}

	images := BuildFinalReviewImages(draft, nil, nil, []string{sourceImage})

	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2 (%+v)", len(images), images)
	}
	if images[1].URL != sourceImage || images[1].Origin != "source" || !images[1].RequiresReview {
		t.Fatalf("offered source image = %+v, want source provenance", images[1])
	}
}
