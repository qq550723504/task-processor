package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestResolveSDSImagesForSKCUsesSourceSKUAndColor(t *testing.T) {
	t.Parallel()

	skuImages := &common.ImageSet{MainImage: "sku.jpg"}
	colorImages := &common.ImageSet{MainImage: "blue.jpg"}
	pkg := &Package{
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SupplierCode: "SUPPLIER-IGNORED",
				SKUList: []SKUDraft{{
					SupplierSKU: "CURRENT-SKU-A1",
					Attributes:  map[string]string{"source_sds_sku": "SDS-SKU", "Color": "Blue"},
				}},
			}},
		},
	}

	bySKU := map[string]*common.ImageSet{NormalizeSDSImageKey("SDS-SKU"): skuImages}
	byColor := map[string]*common.ImageSet{NormalizeSDSImageKey("Blue"): colorImages}

	if got := ResolveSDSImagesForSKC(pkg, 0, bySKU, byColor); got != skuImages {
		t.Fatalf("ResolveSDSImagesForSKC() = %#v, want SKU image set", got)
	}
}

func TestResolveSDSImagesForSKUFallsBackToColor(t *testing.T) {
	t.Parallel()

	colorImages := &common.ImageSet{MainImage: "red.jpg"}
	sku := &SKUDraft{
		SupplierSKU: "UNKNOWN",
		Attributes:  map[string]string{"Color": "Red"},
	}

	if got := ResolveSDSImagesForSKU(sku, nil, map[string]*common.ImageSet{NormalizeSDSImageKey("red"): colorImages}); got != colorImages {
		t.Fatalf("ResolveSDSImagesForSKU() = %#v, want color image set", got)
	}
}

func TestImageSetFromSDSMockupsAndMergeDeduplicateImages(t *testing.T) {
	t.Parallel()

	base := ImageSetFromSDSMockups([]string{" main.jpg ", "gallery.jpg", "gallery.jpg"}, []string{"src.jpg", "src.jpg"})
	if base == nil || base.MainImage != "main.jpg" || len(base.Gallery) != 1 || base.Gallery[0] != "gallery.jpg" {
		t.Fatalf("ImageSetFromSDSMockups() = %#v, want main plus deduplicated gallery", base)
	}
	if len(base.SourceImages) != 1 || base.SourceImages[0] != "src.jpg" {
		t.Fatalf("source images = %#v, want deduplicated source image", base.SourceImages)
	}

	merged := MergeSDSImageSet(base, &common.ImageSet{MainImage: "gallery.jpg", Gallery: []string{"next.jpg", "main.jpg"}})
	if got, want := merged.Gallery, []string{"gallery.jpg", "next.jpg", "main.jpg"}; len(got) != len(want) {
		t.Fatalf("merged gallery = %#v, want %#v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("merged gallery = %#v, want %#v", got, want)
			}
		}
	}
}
