package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestApplyFinalImageDraftAppliesOrderDeletionAndPreviewRoles(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		FinalSubmissionDraft: &FinalDraft{
			MainImageURL:       "https://cdn.example/main.jpg",
			FinalImageOrder:    []string{"https://cdn.example/main.jpg", "https://cdn.example/gallery-2.jpg"},
			DeletedImageURLs:   []string{"https://cdn.example/delete.jpg"},
			ImageRoleOverrides: map[string]string{"https://cdn.example/gallery-2.jpg": "swatch"},
		},
		DraftPayload: &RequestDraft{
			ImageInfo: &ImageDraft{
				MainImage: "https://cdn.example/old-main.jpg",
				Gallery: []string{
					"https://cdn.example/delete.jpg",
					"https://cdn.example/gallery-1.jpg",
					"https://cdn.example/gallery-2.jpg",
				},
				WhiteBg: "https://cdn.example/white.jpg",
			},
			SKCList: []SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SKUList:      []SKUDraft{{SupplierSKU: "SKU-1"}},
			}},
		},
		PreviewPayload: &sheinproduct.Product{
			ImageInfo: &sheinproduct.ImageInfo{ImageInfoList: []sheinproduct.ImageDetail{
				{ImageURL: "https://cdn.example/delete.jpg", ImageType: 2},
				{ImageURL: "https://cdn.example/gallery-2.jpg", ImageType: 2},
			}},
			SKCList: []sheinproduct.SKC{{SupplierCode: stringPtr("SKC-1")}},
		},
	}

	ApplyFinalImageDraft(pkg)

	if got := pkg.DraftPayload.ImageInfo.MainImage; got != "https://cdn.example/main.jpg" {
		t.Fatalf("draft main image = %q, want final main", got)
	}
	for _, image := range pkg.DraftPayload.ImageInfo.Gallery {
		if image == "https://cdn.example/delete.jpg" {
			t.Fatalf("deleted image remained in gallery: %+v", pkg.DraftPayload.ImageInfo.Gallery)
		}
	}
	if pkg.DraftPayload.SKCList[0].ImageInfo == nil || pkg.DraftPayload.SKCList[0].SKUList[0].MainImage == "" {
		t.Fatalf("draft SKC/SKU images were not filled: %+v", pkg.DraftPayload.SKCList[0])
	}
	if len(pkg.PreviewPayload.ImageInfo.ImageInfoList) != 1 {
		t.Fatalf("preview images = %+v, want deleted image removed", pkg.PreviewPayload.ImageInfo.ImageInfoList)
	}
	if got := pkg.PreviewPayload.ImageInfo.ImageInfoList[0]; got.ImageType != 6 || got.MarketingMainImage {
		t.Fatalf("preview role image = %+v, want swatch image type", got)
	}
	if len(pkg.PreviewPayload.SKCList[0].ImageInfo.ImageInfoList) == 0 {
		t.Fatal("preview SKC image info was not filled from draft")
	}
}

func TestNormalizeImageRoleOverridesKeepsAcceptedRoles(t *testing.T) {
	t.Parallel()

	roles := NormalizeImageRoleOverrides(map[string]string{
		" https://cdn.example/main.jpg ": " MAIN ",
		"https://cdn.example/skc.jpg":    "skc",
		"https://cdn.example/nope.jpg":   "invalid",
		" ":                              "swatch",
	})

	if roles["https://cdn.example/main.jpg"] != "main" {
		t.Fatalf("main role = %q, want normalized main", roles["https://cdn.example/main.jpg"])
	}
	if roles["https://cdn.example/skc.jpg"] != "skc" {
		t.Fatalf("skc role = %q, want skc", roles["https://cdn.example/skc.jpg"])
	}
	if _, ok := roles["https://cdn.example/nope.jpg"]; ok {
		t.Fatalf("invalid role kept: %#v", roles)
	}
}

func stringPtr(value string) *string {
	return &value
}
