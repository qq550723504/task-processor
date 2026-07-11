package sdspod

import (
	"reflect"
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestApplyCanonicalNilProductIsNoOp(t *testing.T) {
	if ApplyCanonical(nil, CanonicalMetadata{ProductName: "Clock"}) {
		t.Fatal("ApplyCanonical(nil) = true, want false")
	}
}

func wantTrace(detail string, confidence float64) canonical.FieldTrace {
	return canonical.FieldTrace{
		Sources: []canonical.Source{{
			Type:   canonical.SourceDerived,
			Detail: detail,
		}},
		Confidence:  confidence,
		IsInferred:  false,
		NeedsReview: false,
	}
}

func TestApplyCanonicalMapsTitleIdentityAndStyle(t *testing.T) {
	product := &canonical.Product{
		Title: "Old title",
		Attributes: map[string]canonical.Attribute{
			"sku": {Value: "old"},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-1"},
			{SKU: "SKU-2", Attributes: map[string]canonical.Attribute{}},
		},
	}
	metadata := CanonicalMetadata{
		ProductName:  "  Rendered Clock  ",
		ProductSKU:   " PARENT-1 ",
		VariantSKU:   " CHILD-1 ",
		VariantSize:  " 40x60cm ",
		VariantColor: " White ",
		StyleName:    " Style B6C753EB ",
		Attributes: map[string]string{
			"material": " Cotton ",
			"sku":      "fallback-sku",
		},
	}

	if !ApplyCanonical(product, metadata) {
		t.Fatal("ApplyCanonical() = false, want true")
	}
	if product.Title != "Rendered Clock" {
		t.Fatalf("Title = %q", product.Title)
	}
	wantValues := map[string]string{
		"material":      "Cotton",
		"sku":           "PARENT-1",
		"product_sku":   "PARENT-1",
		"variant_sku":   "CHILD-1",
		"variant_size":  "40x60cm",
		"variant_color": "White",
	}
	for key, want := range wantValues {
		if got := product.Attributes[key].Value; got != want {
			t.Fatalf("attribute %s = %q, want %q", key, got, want)
		}
		if !reflect.DeepEqual(product.Attributes[key].Trace, wantTrace(
			"SDS design product identity", 0.96)) {
			t.Fatalf("attribute %s trace = %+v", key, product.Attributes[key].Trace)
		}
	}
	for i := range product.Variants {
		attr := product.Variants[i].Attributes["ai_style"]
		if attr.Value != "Style B6C753EB" {
			t.Fatalf("variant[%d] ai_style = %q", i, attr.Value)
		}
		if !reflect.DeepEqual(attr.Trace,
			wantTrace("SDS studio AI style dimension", 0.94)) {
			t.Fatalf("variant[%d] style trace = %+v", i, attr.Trace)
		}
	}
	if !reflect.DeepEqual(product.FieldTraces["title"],
		wantTrace("SDS design product detail", 0.96)) {
		t.Fatalf("title trace = %+v", product.FieldTraces["title"])
	}
	if !reflect.DeepEqual(product.FieldTraces["attributes"],
		wantTrace("SDS design product identity", 0.96)) {
		t.Fatalf("attributes trace = %+v", product.FieldTraces["attributes"])
	}
}

func TestApplyCanonicalTitleIdentityAndStyleAreIdempotent(t *testing.T) {
	product := &canonical.Product{Variants: []canonical.Variant{{SKU: "SKU"}}}
	metadata := CanonicalMetadata{
		ProductName: "Clock",
		ProductSKU:  "PARENT",
		StyleName:   "Style A1",
	}
	if !ApplyCanonical(product, metadata) {
		t.Fatal("first ApplyCanonical() = false")
	}
	if ApplyCanonical(product, metadata) {
		t.Fatal("second ApplyCanonical() = true, want false")
	}
}

func TestApplyCanonicalEmptyMetadataIsNoOp(t *testing.T) {
	product := &canonical.Product{Title: "Existing"}
	if ApplyCanonical(product, CanonicalMetadata{}) {
		t.Fatal("ApplyCanonical(empty) = true, want false")
	}
	if product.Title != "Existing" {
		t.Fatalf("Title = %q, want Existing", product.Title)
	}
}
