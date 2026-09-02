package listingkit

import (
	"context"
	"errors"
	"reflect"
	"testing"

	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

type approvedAssetReaderFunc func(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error)

func (f approvedAssetReaderFunc) GetApprovedInventory(ctx context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return f(ctx, scope)
}

func TestWorkflowRequiresApprovedAssets(t *testing.T) {
	wantScope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	phase := standardWorkflowAssetPhase{approvedAssets: approvedAssetReaderFunc(func(_ context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
		if !reflect.DeepEqual(scope, wantScope) {
			t.Fatalf("scope = %+v, want %+v", scope, wantScope)
		}
		return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
	})}

	_, err := phase.run(context.Background(), wantScope)
	if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		t.Fatalf("run() error = %v, want ErrApprovedAssetsNotReady", err)
	}
}

func TestWorkflowRejectsApprovedGalleryWithoutMain(t *testing.T) {
	scope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	phase := standardWorkflowAssetPhase{approvedAssets: approvedAssetReaderFunc(func(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
		return productasset.ApprovedAssetInventory{
			Scope: scope,
			Assets: []productasset.ApprovedAsset{{
				ID: "gallery-1", Role: productasset.RoleGallery, URL: "https://cdn.example/gallery.png",
			}},
		}, nil
	})}

	_, err := phase.run(context.Background(), scope)
	if !errors.Is(err, productasset.ErrApprovedAssetsNotReady) {
		t.Fatalf("run() error = %v, want ErrApprovedAssetsNotReady", err)
	}
}

func TestWorkflowReadsApprovedMainWithoutSourceFallback(t *testing.T) {
	scope := productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"}
	want := productasset.ApprovedAssetInventory{
		Scope: scope,
		Assets: []productasset.ApprovedAsset{{
			ID: "main-1", Role: productasset.RoleMain, URL: "https://cdn.example/main.png", Operations: []string{"approved"},
		}},
	}
	phase := standardWorkflowAssetPhase{approvedAssets: approvedAssetReaderFunc(func(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
		return want, nil
	})}

	got, err := phase.run(context.Background(), scope)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("run() = %+v, want %+v", got, want)
	}
	got.Assets[0].Operations[0] = "mutated"
	if want.Assets[0].Operations[0] != "approved" {
		t.Fatalf("reader inventory mutated through result: %+v", want)
	}
}

func TestAssemblerUsesOnlyApprovedAssetImages(t *testing.T) {
	snapshot := catalog.ProductSnapshot{
		Title:  "Bottle",
		Images: []catalog.Image{{URL: "https://source.example/source.png", Role: "main"}},
		Variants: []catalog.Variant{{
			SKU: "SKU-1", Images: []catalog.Image{{URL: "https://source.example/variant.png"}},
		}},
	}
	inventory := productasset.ApprovedAssetInventory{
		Scope: productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"},
		Assets: []productasset.ApprovedAsset{
			{ID: "gallery-1", Role: productasset.RoleGallery, URL: "https://cdn.example/gallery.png"},
			{ID: "main-1", Role: productasset.RoleMain, URL: "https://cdn.example/main.png"},
		},
	}
	task := &Task{TenantID: "tenant-a", Request: &GenerateRequest{ProductKey: "product-1"}}

	result := NewAssembler(nil).Assemble(task, &snapshot, &inventory)

	if result.CanonicalProduct == nil {
		t.Fatal("canonical product = nil")
	}
	if len(result.CanonicalProduct.Images) != 2 {
		t.Fatalf("canonical images = %#v, want approved projection", result.CanonicalProduct.Images)
	}
	if result.CanonicalProduct.Images[0].URL != "https://cdn.example/main.png" || result.CanonicalProduct.Images[0].Role != string(productasset.RoleMain) {
		t.Fatalf("canonical main = %#v, want approved main", result.CanonicalProduct.Images[0])
	}
	if result.CanonicalProduct.Images[1].URL != "https://cdn.example/gallery.png" || result.CanonicalProduct.Images[1].Role != string(productasset.RoleGallery) {
		t.Fatalf("canonical gallery = %#v, want approved gallery", result.CanonicalProduct.Images[1])
	}
}
