package publishing

import "testing"

func TestFindVariantImageSetPrefersSKUOverColor(t *testing.T) {
	t.Parallel()

	skuSet := VariantImageSet{VariantSKU: "SDS-SKU", ImageURLs: []string{"sku.jpg"}}
	colorSet := VariantImageSet{Color: "Blue", ImageURLs: []string{"blue.jpg"}}

	got, ok := FindVariantImageSet(
		VariantImageSKCInput{
			SKUCandidates:   []string{"SDS-SKU"},
			ColorCandidates: []string{"Blue"},
		},
		map[string]VariantImageSet{NormalizeVariantImageKey("Blue"): colorSet},
		map[string]VariantImageSet{NormalizeVariantImageKey("SDS-SKU"): skuSet},
	)
	if !ok || got.ImageURLs[0] != "sku.jpg" {
		t.Fatalf("FindVariantImageSet() = %+v, %v; want source SKU image set", got, ok)
	}
}

func TestFindVariantImageSetFallsBackToColor(t *testing.T) {
	t.Parallel()

	colorSet := VariantImageSet{Color: "Green", ImageURLs: []string{"green.jpg"}}

	got, ok := FindVariantImageSet(
		VariantImageSKCInput{ColorCandidates: []string{" Green "}},
		map[string]VariantImageSet{NormalizeVariantImageKey("green"): colorSet},
		nil,
	)
	if !ok || got.ImageURLs[0] != "green.jpg" {
		t.Fatalf("FindVariantImageSet() = %+v, %v; want color image set", got, ok)
	}
}
