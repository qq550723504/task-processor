package shein

import "testing"

func TestFindVariantImageSetForRequestSKCUsesSourceSKUBeforeColor(t *testing.T) {
	t.Parallel()

	skuSet := VariantImageSet{VariantSKU: "SDS-SKU", ImageURLs: []string{"sku.jpg"}}
	colorSet := VariantImageSet{Color: "Blue", ImageURLs: []string{"blue.jpg"}}
	skc := SKCRequestDraft{
		SaleAttribute: &ResolvedSaleAttribute{Value: "Blue"},
		SKUList: []SKUDraft{{
			SupplierSKU: "CURRENT-SKU-A1",
			Attributes:  map[string]string{"source_sds_sku": "SDS-SKU", "Color": "Blue"},
		}},
	}

	got, ok := FindVariantImageSetForRequestSKC(
		skc,
		map[string]VariantImageSet{NormalizeVariantImageKey("Blue"): colorSet},
		map[string]VariantImageSet{NormalizeVariantImageKey("SDS-SKU"): skuSet},
	)
	if !ok || got.ImageURLs[0] != "sku.jpg" {
		t.Fatalf("FindVariantImageSetForRequestSKC() = %+v, %v; want source SKU image set", got, ok)
	}
}

func TestFindVariantImageSetForPackageSKCFallsBackToColor(t *testing.T) {
	t.Parallel()

	colorSet := VariantImageSet{Color: "Green", ImageURLs: []string{"green.jpg"}}
	skc := SKCPackage{
		Attributes: map[string]string{"Color": "Green"},
	}

	got, ok := FindVariantImageSetForPackageSKC(
		skc,
		map[string]VariantImageSet{NormalizeVariantImageKey("green"): colorSet},
		nil,
	)
	if !ok || got.ImageURLs[0] != "green.jpg" {
		t.Fatalf("FindVariantImageSetForPackageSKC() = %+v, %v; want color image set", got, ok)
	}
}
