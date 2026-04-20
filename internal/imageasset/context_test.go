package imageasset

import (
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
)

func TestBuildContext_UsesInventoryMainAndSelection(t *testing.T) {
	product := &catalog.Product{
		Title:        " Demo Tee ",
		Brand:        " ACME ",
		CategoryPath: []string{"Apparel", "Tops"},
		Images:       []catalog.Image{{URL: "https://img/source-1.jpg", Role: "main"}},
		Variants:     []catalog.Variant{{Images: []catalog.Image{{URL: "https://img/source-2.jpg", Role: "detail"}}}},
	}
	inv := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, Origin: asset.OriginSource, URL: "https://img/source-1.jpg"},
			{ID: "main-1", Kind: asset.KindMainImage, Origin: asset.OriginDerived, URL: "https://img/main-1.jpg", Metadata: map[string]string{"recipe": "main"}},
		},
		Summary: &asset.InventorySummary{
			MainAssetID:      "main-1",
			SelectedAssetIDs: []string{"main-1"},
		},
	}

	ctx := BuildContext(product, inv)

	if ctx.Product.Title != "Demo Tee" {
		t.Fatalf("expected trimmed title, got %q", ctx.Product.Title)
	}
	if ctx.Product.Brand != "ACME" {
		t.Fatalf("expected trimmed brand, got %q", ctx.Product.Brand)
	}
	if len(ctx.Sources) != 2 {
		t.Fatalf("expected 2 deduplicated sources, got %d", len(ctx.Sources))
	}
	if ctx.Main == nil || ctx.Main.ID != "main-1" {
		t.Fatalf("expected main asset main-1, got %#v", ctx.Main)
	}

	var foundMain bool
	for _, item := range ctx.Assets {
		if item.ID == "main-1" {
			foundMain = true
			if !item.Selected || !item.Main {
				t.Fatalf("expected main-1 selected/main=true, got selected=%t main=%t", item.Selected, item.Main)
			}
			if item.Metadata["recipe"] != "main" {
				t.Fatalf("expected metadata recipe preserved, got %#v", item.Metadata)
			}
		}
	}
	if !foundMain {
		t.Fatal("expected to find main-1 in assets")
	}
}

func TestBuildContext_FallbacksToMainKindWhenSummaryMissing(t *testing.T) {
	inv := &asset.Inventory{Records: []asset.AssetRecord{
		{ID: "a1", Kind: asset.KindGalleryImage, Origin: asset.OriginGenerated, URL: "https://img/a1.jpg"},
		{ID: "m1", Kind: asset.KindMainImage, Origin: asset.OriginDerived, URL: "https://img/m1.jpg"},
	}}

	ctx := BuildContext(nil, inv)

	if ctx.Main == nil || ctx.Main.ID != "m1" {
		t.Fatalf("expected fallback main asset m1, got %#v", ctx.Main)
	}
}
