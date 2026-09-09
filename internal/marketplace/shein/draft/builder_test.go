package draft

import (
	"context"
	"testing"

	"task-processor/internal/listing/record"
	contract "task-processor/internal/marketplace/validator"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	sheinpub "task-processor/internal/publishing/shein"

	"github.com/stretchr/testify/require"
)

func TestBuilderUsesOnlyControlledExactApprovedAssets(t *testing.T) {
	snapshot := catalog.ProductSnapshot{
		Title:    "Controlled bottle",
		Images:   []catalog.Image{{URL: "https://source.invalid/product.jpg"}},
		Variants: []catalog.Variant{{SKU: "sku-1", Images: []catalog.Image{{URL: "https://source.invalid/variant.jpg"}}}},
	}
	inventory := productasset.ApprovedAssetInventory{
		Scope: productasset.InventoryScope{TenantID: "200", ProductKey: "product", TargetPlatform: "shein", SourceSnapshotVersion: 7},
		Assets: []productasset.ApprovedAsset{
			{ID: "approved-main", RunID: "run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://controlled.invalid/main.jpg"},
			{ID: "approved-gallery", RunID: "run", PlanRevision: 1, SlotID: "gallery", Attempt: 1, Role: productasset.RoleGallery, URL: "https://controlled.invalid/gallery.jpg"},
		},
	}
	input := record.Input{ProductKey: "product", SnapshotVersion: 7, StoreID: "11111111-1111-4111-8111-111111111111", Country: "US", Language: "en", Action: contract.SaveDraft}

	raw, err := (Builder{}).Build(context.Background(), snapshot, inventory, input)
	require.NoError(t, err)
	for range 10 {
		again, buildErr := (Builder{}).Build(context.Background(), snapshot, inventory, input)
		require.NoError(t, buildErr)
		require.Equal(t, raw, again, "pure draft mapping must be byte-deterministic")
	}
	pkg, err := sheinpub.DecodePersistedPackageStrict(raw)
	require.NoError(t, err)
	require.NotNil(t, pkg.Images)
	require.Equal(t, "https://controlled.invalid/main.jpg", pkg.Images.MainImage)
	require.Contains(t, pkg.Images.Gallery, "https://controlled.invalid/gallery.jpg")
	require.NotContains(t, string(raw), "https://source.invalid/product.jpg")
	require.NotContains(t, string(raw), "https://source.invalid/variant.jpg")
}
