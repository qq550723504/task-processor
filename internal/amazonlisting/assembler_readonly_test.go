package amazonlisting

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func TestBuildDraftRequiresSnapshotAndApprovedMainAsset(t *testing.T) {
	_, err := NewAssembler().Build(DraftInput{
		Snapshot: catalog.ProductSnapshot{Title: "Bottle"},
		ApprovedAssets: productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{
			{ID: "gallery-1", Role: productasset.RoleGallery, URL: "https://cdn.example.com/gallery.jpg"},
		}},
	})
	if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		t.Fatalf("Build() error = %v, want ErrApprovedAssetsNotReady", err)
	}
}

func TestBuildDraftUsesSnapshotFactsAndOnlyApprovedImages(t *testing.T) {
	draft, err := NewAssembler().Build(DraftInput{
		TaskID: "task-1",
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			Language:    "en_US",
			BrandHint:   "Selling Brand",
		},
		Snapshot: catalog.ProductSnapshot{
			Title:         "Insulated Bottle",
			Brand:         "Supplier Brand",
			CategoryPath:  []string{"Home", "Drinkware"},
			Description:   "Vacuum insulated stainless steel bottle.",
			SellingPoints: []string{"Leak resistant", "Keeps drinks cold"},
			SEOKeywords:   []string{"insulated bottle"},
			Attributes:    []catalog.Attribute{{Name: "material", Value: "stainless steel"}},
			Images:        []catalog.Image{{URL: "https://source.example.com/raw.jpg", Role: "source"}},
		},
		ApprovedAssets: productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{
			{ID: "main-1", Role: productasset.RoleMain, URL: "https://cdn.example.com/main.jpg"},
			{ID: "white-1", Role: productasset.RoleWhiteBackground, URL: "https://cdn.example.com/white.jpg"},
			{ID: "gallery-1", Role: productasset.RoleGallery, URL: "https://cdn.example.com/gallery.jpg"},
		}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if draft.Title != "Insulated Bottle" || draft.Brand != "Selling Brand" {
		t.Fatalf("draft facts = title %q brand %q", draft.Title, draft.Brand)
	}
	if !reflect.DeepEqual(draft.CategoryPath, []string{"Home", "Drinkware"}) {
		t.Fatalf("category path = %v", draft.CategoryPath)
	}
	if draft.Attributes["material"] != "stainless steel" {
		t.Fatalf("material = %q", draft.Attributes["material"])
	}
	if draft.Images == nil || draft.Images.MainImage != "https://cdn.example.com/main.jpg" || draft.Images.WhiteBgImage != "https://cdn.example.com/white.jpg" {
		t.Fatalf("approved images = %+v", draft.Images)
	}
	if !reflect.DeepEqual(draft.Images.GalleryImages, []string{"https://cdn.example.com/gallery.jpg"}) {
		t.Fatalf("gallery images = %v", draft.Images.GalleryImages)
	}
	raw, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if strings.Contains(string(raw), "https://source.example.com/raw.jpg") {
		t.Fatalf("draft leaked a source image: %s", raw)
	}
}
