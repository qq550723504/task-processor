package shein

import (
	"strings"
	"testing"

	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestFinalSubmitImagesReadyRequiresPublishSKCImageAndSwatch(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		FinalSubmissionDraft: &FinalDraft{MainImageURL: "https://cdn.example/main.jpg"},
		DraftPayload: &RequestDraft{
			ImageInfo: &ImageDraft{
				MainImage: "https://cdn.example/main.jpg",
				Gallery:   []string{"https://cdn.example/gallery.jpg"},
			},
			SKCList: []SKCRequestDraft{{}, {}},
		},
	}

	ready, message := FinalSubmitImagesReady(pkg, "publish")
	if ready || !strings.Contains(message, "SKC") {
		t.Fatalf("FinalSubmitImagesReady(no skc) = (%v, %q), want SKC blocker", ready, message)
	}

	pkg.DraftPayload.SKCList[0].ImageInfo = &ImageDraft{MainImage: "https://cdn.example/skc.jpg"}
	ready, message = FinalSubmitImagesReady(pkg, "publish")
	if !ready {
		t.Fatalf("FinalSubmitImagesReady(with skc) = (%v, %q), want ready", ready, message)
	}
}

func TestFinalSubmitImagesReadyAllowsSaveDraftWithoutSKCImage(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		FinalSubmissionDraft: &FinalDraft{MainImageURL: "https://cdn.example/main.jpg"},
		DraftPayload: &RequestDraft{
			ImageInfo: &ImageDraft{
				MainImage: "https://cdn.example/main.jpg",
				Gallery:   []string{"https://cdn.example/gallery.jpg"},
			},
			SKCList: []SKCRequestDraft{{}},
		},
	}

	ready, message := FinalSubmitImagesReady(pkg, "save_draft")
	if !ready {
		t.Fatalf("FinalSubmitImagesReady(save_draft) = (%v, %q), want ready", ready, message)
	}
}

func TestSubmitPricingReadyRequiresPositiveSKUAndSitePrices(t *testing.T) {
	t.Parallel()

	pkg := &Package{DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
		SKUList: []SKUDraft{{
			BasePrice:     "12.34",
			SitePriceList: []SitePrice{{SubSite: "US", BasePrice: "15.99"}},
		}},
	}}}}

	if !SubmitPricingReady(pkg) {
		t.Fatal("SubmitPricingReady() = false, want true")
	}
	pkg.DraftPayload.SKCList[0].SKUList[0].SitePriceList[0].BasePrice = "0"
	if SubmitPricingReady(pkg) {
		t.Fatal("SubmitPricingReady(zero site price) = true, want false")
	}
}

func TestFinalReviewReadyRequiresConfirmationForPublish(t *testing.T) {
	t.Parallel()

	pkg := &Package{FinalSubmissionDraft: &FinalDraft{}}

	if FinalReviewReady(pkg, "publish") {
		t.Fatal("FinalReviewReady(publish, unconfirmed) = true, want false")
	}
	pkg.FinalSubmissionDraft.Confirmed = true
	if !FinalReviewReady(pkg, "publish") {
		t.Fatal("FinalReviewReady(publish, confirmed) = false, want true")
	}
}

func TestFinalReviewReadyAllowsSaveDraft(t *testing.T) {
	t.Parallel()

	pkg := &Package{FinalSubmissionDraft: &FinalDraft{}}

	if !FinalReviewReady(pkg, " save_draft ") {
		t.Fatal("FinalReviewReady(save_draft) = false, want true")
	}
	if got := FinalReviewMessage("save_draft"); got == "" || got == FinalReviewMessage("publish") {
		t.Fatalf("FinalReviewMessage(save_draft) = %q, want draft-specific message", got)
	}
}

func TestHasSubmitImageChecksPackageDraftAndPreviewImages(t *testing.T) {
	t.Parallel()

	if HasSubmitImage(&Package{Images: &common.ImageSet{WhiteBgImage: " https://cdn.example/white.jpg "}}) != true {
		t.Fatal("HasSubmitImage(package images) = false, want true")
	}
	if HasSubmitImage(&Package{DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
		SKUList: []SKUDraft{{MainImage: "https://cdn.example/sku.jpg"}},
	}}}}) != true {
		t.Fatal("HasSubmitImage(draft sku image) = false, want true")
	}
	if HasSubmitImage(&Package{PreviewPayload: &sheinproduct.Product{SKCList: []sheinproduct.SKC{{
		ImageInfo: sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{{ImageURL: "https://cdn.example/skc.jpg"}}},
	}}}}) != true {
		t.Fatal("HasSubmitImage(preview skc image) = false, want true")
	}
}
