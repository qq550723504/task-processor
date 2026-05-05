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

func TestApplySheinVariantImageCoverageGuardMarksBlockedWithoutClearingImages(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/shared-main.jpg"
	task := &Task{
		Request: &GenerateRequest{
			Options: &GenerateOptions{
				SheinStudio: &SheinStudioOptions{},
			},
		},
		Result: &ListingKitResult{
			SDSSync: &SDSSyncSummary{
				Status: "failed",
				Error:  "SDS render failed for selected color variants: green",
				VariantResults: []SDSSyncSummary{
					{VariantColor: "red", Status: "completed", MockupImageURLs: []string{mainImage}},
					{VariantColor: "green", Status: "failed"},
				},
			},
			Summary: &GenerationSummary{},
		},
	}
	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{
				{
					SupplierCode: "RED",
					ImageInfo:    &sheinpub.ImageDraft{MainImage: mainImage},
					SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "RED-1", MainImage: mainImage, Attributes: map[string]string{"Color": "red"}}},
				},
				{
					SupplierCode: "GREEN",
					ImageInfo:    &sheinpub.ImageDraft{MainImage: mainImage},
					SKUList:      []sheinpub.SKUDraft{{SupplierSKU: "GREEN-1", MainImage: mainImage, Attributes: map[string]string{"Color": "green"}}},
				},
			},
		},
		PreviewProduct: &sheinproduct.Product{
			SKCList: []sheinproduct.SKC{
				{SupplierCode: testStringPtr("RED"), ImageInfo: *sheinImageInfo([]string{mainImage})},
				{SupplierCode: testStringPtr("GREEN"), ImageInfo: *sheinImageInfo([]string{mainImage})},
			},
		},
		FinalDraft: &sheinpub.FinalDraft{Confirmed: true},
	}

	blocked := applySheinVariantImageCoverageGuard(task, pkg)
	if !blocked {
		t.Fatal("expected coverage guard to block incomplete variant coverage")
	}
	if got := pkg.RequestDraft.SKCList[0].ImageInfo.MainImage; got != mainImage {
		t.Fatalf("first skc main image = %q, want preserved shared image", got)
	}
	if got := pkg.RequestDraft.SKCList[1].ImageInfo.MainImage; got != mainImage {
		t.Fatalf("second skc main image = %q, want preserved shared image", got)
	}
	if pkg.Metadata[sheinVariantImageCoverageStatusKey] != "blocked" {
		t.Fatalf("metadata = %#v, want blocked coverage status", pkg.Metadata)
	}
	readiness := buildSheinSubmitReadinessForAction(pkg, "publish")
	if readiness == nil || readiness.Ready {
		t.Fatalf("readiness = %+v, want blocked readiness", readiness)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "variant_image_coverage" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("blocking items = %+v, want variant image coverage blocker", readiness.BlockingItems)
	}
}

func testStringPtr(value string) *string {
	return &value
}
