package amazonlisting

import (
	"reflect"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func TestAssemblerUsesTargetCategoryHintPath(t *testing.T) {
	assembled, err := NewAssembler().Build(DraftInput{
		TaskID:  "task-1",
		Request: &GenerateRequest{Marketplace: "amazon", Country: "US", TargetCategoryHint: "Electronics > Headphones"},
		Snapshot: catalog.ProductSnapshot{
			Title:        "Wireless Headphones",
			Description:  "Over-ear wireless headphones with long battery life.",
			CategoryPath: []string{"Consumer Goods", "Audio"},
		},
		ApprovedAssets: productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{
			{ID: "main-1", Role: productasset.RoleMain, URL: "https://cdn.example.com/main.jpg"},
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if assembled.ProductType != "Headphones" {
		t.Fatalf("expected product type from hint, got %q", assembled.ProductType)
	}
	if expected := []string{"Electronics", "Headphones"}; !reflect.DeepEqual(assembled.CategoryPath, expected) {
		t.Fatalf("expected category path %v, got %v", expected, assembled.CategoryPath)
	}
}

func TestAssemblerKeepsSnapshotCategoryWhenTargetCategoryHintMissing(t *testing.T) {
	assembled, err := NewAssembler().Build(DraftInput{
		TaskID:  "task-2",
		Request: &GenerateRequest{Marketplace: "amazon", Country: "US"},
		Snapshot: catalog.ProductSnapshot{
			Title:        "Ceramic Mug",
			Description:  "A ceramic mug for coffee and tea.",
			CategoryPath: []string{"Home & Kitchen", "Drinkware"},
		},
		ApprovedAssets: productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{
			{ID: "main-1", Role: productasset.RoleMain, URL: "https://cdn.example.com/main.jpg"},
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if assembled.ProductType != "Drinkware" {
		t.Fatalf("expected product type from snapshot category, got %q", assembled.ProductType)
	}
	if expected := []string{"Home & Kitchen", "Drinkware"}; !reflect.DeepEqual(assembled.CategoryPath, expected) {
		t.Fatalf("expected category path %v, got %v", expected, assembled.CategoryPath)
	}
}
