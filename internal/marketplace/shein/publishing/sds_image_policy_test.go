package publishing

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestResolveSDSImagesUsesSKUBeforeColor(t *testing.T) {
	t.Parallel()

	skuImages := &common.ImageSet{MainImage: "sku.jpg"}
	colorImages := &common.ImageSet{MainImage: "color.jpg"}

	got := ResolveSDSImages(
		SDSImageLookupInput{
			SKUCandidates:   []string{"SDS-SKU"},
			ColorCandidates: []string{"Blue"},
		},
		map[string]*common.ImageSet{NormalizeSDSImageKey("SDS-SKU"): skuImages},
		map[string]*common.ImageSet{NormalizeSDSImageKey("Blue"): colorImages},
	)
	if got != skuImages {
		t.Fatalf("ResolveSDSImages() = %#v, want SKU image set", got)
	}
}

func TestResolveSDSImagesFallsBackToColor(t *testing.T) {
	t.Parallel()

	colorImages := &common.ImageSet{MainImage: "red.jpg"}

	got := ResolveSDSImages(
		SDSImageLookupInput{ColorCandidates: []string{"red"}},
		nil,
		map[string]*common.ImageSet{NormalizeSDSImageKey("red"): colorImages},
	)
	if got != colorImages {
		t.Fatalf("ResolveSDSImages() = %#v, want color image set", got)
	}
}

func TestImageSetFromSDSMockupsAndMergeDeduplicateImages(t *testing.T) {
	t.Parallel()

	base := ImageSetFromSDSMockups([]string{" main.jpg ", "gallery.jpg", "gallery.jpg"}, []string{"src.jpg", "src.jpg"})
	if base == nil || base.MainImage != "main.jpg" || len(base.Gallery) != 1 || base.Gallery[0] != "gallery.jpg" || len(base.SourceImages) != 1 {
		t.Fatalf("ImageSetFromSDSMockups() = %#v, want main plus deduplicated gallery", base)
	}

	merged := MergeSDSImageSet(base, &common.ImageSet{MainImage: "gallery.jpg", Gallery: []string{"next.jpg", "main.jpg"}})
	if len(merged.Gallery) != 3 || merged.Gallery[0] != "gallery.jpg" || merged.Gallery[1] != "next.jpg" || merged.Gallery[2] != "main.jpg" {
		t.Fatalf("MergeSDSImageSet().Gallery = %#v, want unique appended gallery", merged.Gallery)
	}
}

func TestSourceSDSSKUFromSupplierSKU(t *testing.T) {
	t.Parallel()

	if got := SourceSDSSKUFromSupplierSKU(" SDS-SKU-A1 "); got != "SDS-SKU" {
		t.Fatalf("SourceSDSSKUFromSupplierSKU() = %q, want SDS-SKU", got)
	}
}
