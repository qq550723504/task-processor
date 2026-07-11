package sdspod

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestApplyCanonicalNilProductIsNoOp(t *testing.T) {
	if ApplyCanonical(nil, CanonicalMetadata{ProductName: "Clock"}) {
		t.Fatal("ApplyCanonical(nil) = true, want false")
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
