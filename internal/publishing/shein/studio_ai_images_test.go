package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestReplaceImagesWithAIProductImagesUpdatesDraftSKCAndPreview(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		Images: &common.ImageSet{MainImage: "old.jpg"},
		RequestDraft: &RequestDraft{
			ImageInfo: BuildImageDraft(&common.ImageSet{MainImage: "old.jpg"}),
			SKCList: []SKCRequestDraft{{
				ImageInfo: BuildImageDraft(&common.ImageSet{MainImage: "old.jpg"}),
				SKUList:   []SKUDraft{{MainImage: "old.jpg"}},
			}},
		},
		SkcList: []SKCPackage{{MainImageURL: "old.jpg"}},
	}

	ReplaceImagesWithAIProductImages(pkg, []string{"ai-main.jpg", "ai-gallery.jpg"}, []string{"source.jpg"})

	if pkg.Images == nil || pkg.Images.MainImage != "ai-main.jpg" {
		t.Fatalf("images = %+v, want AI main image", pkg.Images)
	}
	if got := pkg.DraftPayload.SKCList[0].SKUList[0].MainImage; got != "ai-main.jpg" {
		t.Fatalf("draft SKU main image = %q, want AI main image", got)
	}
	if got := pkg.SkcList[0].MainImageURL; got != "ai-main.jpg" {
		t.Fatalf("package SKC main image = %q, want AI main image", got)
	}
	if pkg.PreviewProduct == nil || pkg.PreviewProduct.ImageInfo.ImageInfoList[0].ImageURL != "ai-main.jpg" {
		t.Fatalf("preview product = %+v, want AI main image", pkg.PreviewProduct)
	}
}

func TestAppendAIProductImagesPreservesExistingMainAndAppendsGallery(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		Images: &common.ImageSet{
			MainImage: "rendered-main.jpg",
			Gallery:   []string{"rendered-gallery.jpg"},
		},
		RequestDraft: &RequestDraft{
			ImageInfo: BuildImageDraft(&common.ImageSet{
				MainImage: "rendered-main.jpg",
				Gallery:   []string{"rendered-gallery.jpg"},
			}),
			SKCList: []SKCRequestDraft{{
				ImageInfo: BuildImageDraft(&common.ImageSet{
					MainImage: "rendered-main.jpg",
					Gallery:   []string{"rendered-gallery.jpg"},
				}),
			}},
		},
	}

	AppendAIProductImages(pkg, []string{"ai-gallery.jpg", "rendered-gallery.jpg"}, []string{"source.jpg"})

	if got := pkg.Images.MainImage; got != "rendered-main.jpg" {
		t.Fatalf("main image = %q, want existing main", got)
	}
	if got, want := pkg.Images.Gallery, []string{"rendered-gallery.jpg", "ai-gallery.jpg"}; len(got) != len(want) {
		t.Fatalf("gallery = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("gallery = %#v, want %#v", got, want)
			}
		}
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product missing")
	}
}
