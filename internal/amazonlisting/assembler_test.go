package amazonlisting

import (
	"reflect"
	"testing"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func TestAssemblerUsesTargetCategoryHintPath(t *testing.T) {
	assembled := NewAssembler().Assemble(&Task{
		ID: "task-1",
		Request: &GenerateRequest{
			Marketplace:        "amazon",
			Country:            "US",
			TargetCategoryHint: "Electronics > Headphones",
		},
	}, &productenrich.CanonicalProduct{
		Title:       "Wireless Headphones",
		Description: "Over-ear wireless headphones with long battery life.",
		CategoryPath: []string{"Consumer Goods", "Audio"},
	}, nil)

	if assembled.ProductType != "Headphones" {
		t.Fatalf("expected product type from hint, got %q", assembled.ProductType)
	}

	expected := []string{"Electronics", "Headphones"}
	if !reflect.DeepEqual(assembled.CategoryPath, expected) {
		t.Fatalf("expected category path %v, got %v", expected, assembled.CategoryPath)
	}
}

func TestAssemblerKeepsProductCategoryWhenTargetCategoryHintMissing(t *testing.T) {
	assembled := NewAssembler().Assemble(&Task{
		ID: "task-2",
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
		},
	}, &productenrich.CanonicalProduct{
		Title:       "Ceramic Mug",
		Description: "A ceramic mug for coffee and tea.",
		CategoryPath: []string{"Home & Kitchen", "Drinkware"},
	}, nil)

	if assembled.ProductType != "Drinkware" {
		t.Fatalf("expected product type from product category, got %q", assembled.ProductType)
	}

	expected := []string{"Home & Kitchen", "Drinkware"}
	if !reflect.DeepEqual(assembled.CategoryPath, expected) {
		t.Fatalf("expected category path %v, got %v", expected, assembled.CategoryPath)
	}
}

func TestAssemblerCarriesImageIPRiskIntoListingIPRisk(t *testing.T) {
	assembled := NewAssembler().Assemble(&Task{
		ID: "task-3",
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
		},
	}, &productenrich.CanonicalProduct{
		Title:       "Ceramic Mug",
		Description: "A ceramic mug for coffee and tea.",
	}, &productimage.ImageProcessResult{
		IPRisk: &productimage.IPRiskReport{
			Level:   "medium",
			Score:   0.4,
			Reasons: []string{"image contains logo or watermark risk"},
		},
	})

	if assembled.ListingIPRisk == nil {
		t.Fatal("expected listing ip risk to be populated")
	}
	if assembled.ListingIPRisk.Level != "medium" {
		t.Fatalf("listing ip risk level = %q, want medium", assembled.ListingIPRisk.Level)
	}
}
