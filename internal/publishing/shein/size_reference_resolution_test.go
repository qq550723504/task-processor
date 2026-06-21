package shein

import "testing"

func TestResolveRenderedSizeReferenceImagesMapsBySourceMockupIndex(t *testing.T) {
	t.Parallel()

	got := ResolveRenderedSizeReferenceImages(
		[]string{"raw-size.jpg"},
		[]string{"raw-main.jpg", "raw-size.jpg"},
		[]string{"rendered-main.jpg", "rendered-size.jpg"},
	)

	if len(got) != 1 || got[0] != "rendered-size.jpg" {
		t.Fatalf("ResolveRenderedSizeReferenceImages() = %#v, want rendered size image", got)
	}
}

func TestFindSizeReferenceVariantSummaryMatchesIDSKUAndColor(t *testing.T) {
	t.Parallel()

	summaries := []SizeReferenceVariantSummary{
		{VariantID: 101, MockupImageURLs: []string{"by-id.jpg"}},
		{VariantSKU: "SKU-1", MockupImageURLs: []string{"by-sku.jpg"}},
		{VariantColor: "Blue", MockupImageURLs: []string{"by-color.jpg"}},
	}

	if got, ok := FindSizeReferenceVariantSummary(SizeReferenceVariantInput{VariantID: 101}, summaries); !ok || got.MockupImageURLs[0] != "by-id.jpg" {
		t.Fatalf("match by id = %+v, %v", got, ok)
	}
	if got, ok := FindSizeReferenceVariantSummary(SizeReferenceVariantInput{VariantSKU: "sku-1"}, summaries); !ok || got.MockupImageURLs[0] != "by-sku.jpg" {
		t.Fatalf("match by sku = %+v, %v", got, ok)
	}
	if got, ok := FindSizeReferenceVariantSummary(SizeReferenceVariantInput{Color: "blue"}, summaries); !ok || got.MockupImageURLs[0] != "by-color.jpg" {
		t.Fatalf("match by color = %+v, %v", got, ok)
	}
}
