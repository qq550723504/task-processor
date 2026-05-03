package listingkit

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestApplySheinFinalImageDraftRepairsExistingPreviewSKCMainImage(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/main.jpg"
	galleryOne := "https://cdn.example.com/gallery-1.jpg"
	galleryTwo := "https://cdn.example.com/gallery-2.jpg"

	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: &sheinpub.ImageDraft{
				MainImage: mainImage,
				Gallery:   []string{mainImage, galleryOne, galleryTwo},
			},
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "BLACK",
				ImageInfo: &sheinpub.ImageDraft{
					MainImage: mainImage,
					Gallery:   []string{mainImage, galleryOne, galleryTwo},
				},
			}},
		},
		PreviewProduct: &sheinproduct.Product{
			ImageInfo: sheinImageInfo([]string{mainImage, galleryOne, galleryTwo}),
			SKCList: []sheinproduct.SKC{{
				SupplierCode: testStringPtr("BLACK"),
				ImageInfo: sheinproduct.ImageInfo{
					ImageInfoList: []sheinproduct.ImageDetail{
						{ImageType: 2, ImageSort: 2, ImageURL: galleryOne},
						{ImageType: 2, ImageSort: 3, ImageURL: galleryTwo},
					},
				},
			}},
		},
		FinalDraft: &sheinpub.FinalDraft{
			MainImageURL:    mainImage,
			FinalImageOrder: []string{mainImage, galleryOne, galleryTwo},
		},
	}

	applySheinFinalImageDraft(pkg)

	got := pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList
	if len(got) != 3 {
		t.Fatalf("preview skc image count = %d, want 3", len(got))
	}
	if got[0].ImageURL != mainImage || got[0].ImageType != 1 {
		t.Fatalf("preview skc main image = %+v, want main image restored first", got[0])
	}
}

func testStringPtr(value string) *string {
	return &value
}
