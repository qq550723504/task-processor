package sdspod

import (
	"reflect"
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestApplyCanonicalAssignsRenderedVariantImages(t *testing.T) {
	product := &canonical.Product{
		Images: []canonical.Image{{URL: "old", Role: "primary"}},
		Variants: []canonical.Variant{
			{SKU: "BLACK"},
			{SKU: "WHITE"},
		},
	}
	metadata := CanonicalMetadata{
		Variants: []VariantMetadata{
			{
				SKU:    "BLACK",
				Color:  "Black",
				Status: "completed",
				MockupURLs: []string{
					" https://cdn/black-main.jpg ",
					"https://cdn/black-side.jpg",
					"https://cdn/black-main.jpg",
				},
			},
			{
				SKU:        "FAILED",
				Color:      "Red",
				Status:     "failed",
				MockupURLs: []string{"https://cdn/failed.jpg"},
			},
			{
				SKU:        "WHITE",
				Color:      "White",
				Status:     "completed",
				MockupURLs: []string{"https://cdn/white-main.jpg"},
			},
		},
		VariantLookup: []VariantLookup{
			{CanonicalVariantIndex: 0, Keys: []string{"BLACK", "Black"}},
			{CanonicalVariantIndex: 1, Keys: []string{"WHITE", "White"}},
		},
	}

	if !ApplyCanonical(product, metadata) {
		t.Fatal("ApplyCanonical() = false")
	}
	wantProductURLs := []string{
		"https://cdn/black-main.jpg",
		"https://cdn/black-side.jpg",
		"https://cdn/white-main.jpg",
	}
	if got := imageURLs(product.Images); !reflect.DeepEqual(got, wantProductURLs) {
		t.Fatalf("product image URLs = %#v", got)
	}
	if got := imageURLs(product.Variants[0].Images); !reflect.DeepEqual(
		got, wantProductURLs[:2]) {
		t.Fatalf("black image URLs = %#v", got)
	}
	if got := imageURLs(product.Variants[1].Images); !reflect.DeepEqual(
		got, wantProductURLs[2:]) {
		t.Fatalf("white image URLs = %#v", got)
	}
	if product.Images[0].Role != "primary" ||
		product.Images[1].Role != "gallery" {
		t.Fatalf("roles = %+v", product.Images)
	}
	wantTrace := canonicalTrace("SDS rendered mockup images", 0.98)
	if !reflect.DeepEqual(product.FieldTraces["images"], wantTrace) {
		t.Fatalf("images trace = %+v", product.FieldTraces["images"])
	}
	if ApplyCanonical(product, metadata) {
		t.Fatal("second ApplyCanonical() = true, want false")
	}
}

func imageURLs(images []canonical.Image) []string {
	out := make([]string, 0, len(images))
	for _, image := range images {
		out = append(out, image.URL)
	}
	return out
}

func TestApplyCanonicalUsesDefaultImagesAndIgnoresInvalidLookups(t *testing.T) {
	product := &canonical.Product{
		Variants: []canonical.Variant{{SKU: "A"}, {SKU: "B"}},
	}
	metadata := CanonicalMetadata{
		MockupURLs: []string{" main.jpg ", "side.jpg", "main.jpg"},
		VariantLookup: []VariantLookup{
			{CanonicalVariantIndex: -1, Keys: []string{"A"}},
			{CanonicalVariantIndex: 9, Keys: []string{"B"}},
			{CanonicalVariantIndex: 0, Keys: []string{"", "missing"}},
		},
	}
	if !ApplyCanonical(product, metadata) {
		t.Fatal("ApplyCanonical() = false")
	}
	want := []string{"main.jpg", "side.jpg"}
	if !reflect.DeepEqual(imageURLs(product.Images), want) {
		t.Fatalf("product images = %+v", product.Images)
	}
	for i := range product.Variants {
		if !reflect.DeepEqual(imageURLs(product.Variants[i].Images), want) {
			t.Fatalf("variant[%d] images = %+v", i, product.Variants[i].Images)
		}
	}
}
