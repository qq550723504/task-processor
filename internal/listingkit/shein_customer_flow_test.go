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

	sheinpub.ApplyFinalImageDraft(pkg)

	got := pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList
	if len(got) != 3 {
		t.Fatalf("preview skc image count = %d, want 3", len(got))
	}
	if got[0].ImageURL != mainImage || got[0].ImageType != 1 {
		t.Fatalf("preview skc main image = %+v, want main image restored first", got[0])
	}
}

func TestApplySheinFinalImageDraftBackfillsSKCGalleryFromFinalOrder(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/main.jpg"
	galleryOne := "https://cdn.example.com/gallery-1.jpg"
	galleryTwo := "https://cdn.example.com/gallery-2.jpg"
	skcMain := "https://cdn.example.com/skc-main.jpg"

	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: &sheinpub.ImageDraft{
				MainImage: mainImage,
				Gallery:   []string{mainImage, galleryOne, galleryTwo},
			},
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "WHITE",
				ImageInfo: &sheinpub.ImageDraft{
					MainImage: skcMain,
				},
			}},
		},
		PreviewProduct: &sheinproduct.Product{
			ImageInfo: sheinImageInfo([]string{mainImage, galleryOne, galleryTwo}),
			SKCList: []sheinproduct.SKC{{
				SupplierCode: testStringPtr("WHITE"),
				ImageInfo: sheinproduct.ImageInfo{
					ImageInfoList: []sheinproduct.ImageDetail{
						{ImageType: 1, ImageSort: 1, ImageURL: skcMain},
					},
				},
			}},
		},
		FinalDraft: &sheinpub.FinalDraft{
			MainImageURL:    mainImage,
			FinalImageOrder: []string{mainImage, galleryOne, galleryTwo, skcMain},
		},
	}

	sheinpub.ApplyFinalImageDraft(pkg)

	draftImages := pkg.DraftPayload.SKCList[0].ImageInfo
	if draftImages == nil {
		t.Fatal("skc draft image info = nil, want backfilled gallery")
	}
	if len(draftImages.Gallery) < 2 {
		t.Fatalf("skc draft gallery = %+v, want at least two detail images from final order", draftImages.Gallery)
	}

	got := pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList
	if len(got) < 3 {
		t.Fatalf("preview skc image count = %d, want main plus backfilled gallery images", len(got))
	}
	if got[0].ImageURL != skcMain || got[0].ImageType != 1 {
		t.Fatalf("preview skc main image = %+v, want original skc main restored first", got[0])
	}
}

func TestApplySheinFinalImageDraftBackfillsSKCGalleryWhenOnlyMainExists(t *testing.T) {
	t.Parallel()

	topMain := "https://cdn.example.com/top-main.jpg"
	detailOne := "https://cdn.example.com/detail-1.jpg"
	detailTwo := "https://cdn.example.com/detail-2.jpg"
	skcMain := "https://cdn.example.com/skc-main.jpg"

	pkg := &sheinpub.Package{
		RequestDraft: &sheinpub.RequestDraft{
			ImageInfo: &sheinpub.ImageDraft{
				MainImage: topMain,
				Gallery:   []string{topMain, detailOne, detailTwo},
			},
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "WHITE",
				ImageInfo: &sheinpub.ImageDraft{
					MainImage: skcMain,
				},
			}},
		},
		PreviewProduct: &sheinproduct.Product{
			ImageInfo: sheinImageInfo([]string{topMain, detailOne, detailTwo}),
			SKCList: []sheinproduct.SKC{{
				SupplierCode: testStringPtr("WHITE"),
				ImageInfo: sheinproduct.ImageInfo{
					ImageInfoList: []sheinproduct.ImageDetail{
						{ImageType: 1, ImageSort: 1, ImageURL: skcMain},
					},
				},
			}},
		},
		FinalDraft: &sheinpub.FinalDraft{
			Confirmed: true,
		},
	}

	sheinpub.ApplyFinalImageDraft(pkg)

	draftImages := pkg.DraftPayload.SKCList[0].ImageInfo
	if draftImages == nil {
		t.Fatal("skc draft image info = nil, want backfilled gallery")
	}
	if len(draftImages.Gallery) < 2 {
		t.Fatalf("skc draft gallery = %+v, want at least two detail images from top-level final images", draftImages.Gallery)
	}
	if draftImages.Gallery[0] != detailOne || draftImages.Gallery[1] != detailTwo {
		t.Fatalf("skc draft gallery = %+v, want detail images preserved in order", draftImages.Gallery)
	}

	got := pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList
	if len(got) < 3 {
		t.Fatalf("preview skc image count = %d, want main plus backfilled detail images", len(got))
	}
	if got[0].ImageURL != skcMain || got[0].ImageType != 1 {
		t.Fatalf("preview skc main image = %+v, want original skc main restored first", got[0])
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

	blocked := applySheinVariantImageCoverageGuard(task.Result, task.Request, pkg)
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

func TestApplySheinVariantImageCoverageGuardMarksProvidedResultNeedsReview(t *testing.T) {
	t.Parallel()

	mainImage := "https://cdn.example.com/shared-main.jpg"
	req := &GenerateRequest{
		Options: &GenerateOptions{
			SheinStudio: &SheinStudioOptions{},
		},
	}
	result := &ListingKitResult{
		SDSSync: &SDSSyncSummary{
			Status: "failed",
			Error:  "SDS render failed for selected color variants: green",
			VariantResults: []SDSSyncSummary{
				{VariantColor: "red", Status: "completed", MockupImageURLs: []string{mainImage}},
				{VariantColor: "green", Status: "failed"},
			},
		},
		Summary: &GenerationSummary{},
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
	}

	if !applySheinVariantImageCoverageGuard(result, req, pkg) {
		t.Fatal("expected coverage guard to block incomplete variant coverage")
	}
	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want provided result marked needs_review", result.Summary)
	}
	if len(result.ReviewReasons) == 0 {
		t.Fatalf("review reasons = %#v, want blocking reason on provided result", result.ReviewReasons)
	}
}

func testStringPtr(value string) *string {
	return &value
}
