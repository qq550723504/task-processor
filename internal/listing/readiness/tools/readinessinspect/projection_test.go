package readinessinspect

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func productFixture() catalog.PublishedSnapshot {
	return catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"},
		Version: 7, PublicationID: "publication-7", Snapshot: catalog.ProductSnapshot{Title: "Bottle"}}
}

func inventoryFixture() *asset.ApprovedAssetInventory {
	return &asset.ApprovedAssetInventory{Scope: asset.InventoryScope{TenantID: "org-a", ProductKey: "product-1", TargetPlatform: "shein", SourceSnapshotVersion: 7},
		Assets: []asset.ApprovedAsset{{ID: "approved-1", RunID: "run-1", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: "https://controlled.invalid/image.jpg"}}}
}

func TestProjectionScopesReadyAndPreservesUnassessedMarketplace(t *testing.T) {
	wire, err := Project(productFixture(), inventoryFixture(), "shein")
	require.NoError(t, err)
	var out Output
	require.NoError(t, json.Unmarshal(wire, &out))
	require.Equal(t, "ready", out.Status)
	require.Equal(t, "product.inputs", out.Scope)
	require.Equal(t, "7", out.ProductVersion)
	require.Equal(t, "7", out.AssetBinding.ProductVersion)
	require.Equal(t, "not_evaluated", out.Marketplace.Status)
	require.NotEmpty(t, out.Marketplace.Reasons)
	require.NotContains(t, string(wire), "controlled.invalid")
	for range 3 {
		again, repeatErr := Project(productFixture(), inventoryFixture(), "shein")
		require.NoError(t, repeatErr)
		require.Equal(t, wire, again)
	}
}

func TestProjectionUsesDomainMissingAssetAndReviewBlockers(t *testing.T) {
	p := productFixture()
	p.Snapshot.Review = &catalog.ReviewState{NeedsReview: true, Reasons: []string{"missing dimensions"}}
	p.Snapshot.Warnings = []catalog.Warning{{Code: "missing_dimensions", Field: "dimensions", Message: "missing dimensions"}}
	wire, err := Project(p, nil, "shein")
	require.NoError(t, err)
	var out Output
	require.NoError(t, json.Unmarshal(wire, &out))
	require.Equal(t, "blocked", out.Status)
	require.Contains(t, out.Reasons, "approved_assets_not_ready")
	require.Contains(t, out.Reasons, "source_review_required")
	require.Contains(t, string(wire), "missing_dimensions")
	require.Contains(t, out.MissingFacts, "approved_assets")
}

func TestProjectionExplicitTargetUsesNeutralInputRules(t *testing.T) {
	inventory := inventoryFixture()
	inventory.Scope.TargetPlatform = "another-marketplace"
	wire, err := Project(productFixture(), inventory, "another-marketplace")
	require.NoError(t, err)
	var out Output
	require.NoError(t, json.Unmarshal(wire, &out))
	require.Equal(t, "ready", out.Status)
	require.NotEmpty(t, out.InputRuleVersion)
	require.Equal(t, "not_evaluated", out.Marketplace.Status)
}

func TestProjectionRejectsCorruptBindingAndBounds(t *testing.T) {
	for _, mutate := range []func(*asset.ApprovedAssetInventory){
		func(a *asset.ApprovedAssetInventory) { a.Scope.TenantID = "other" },
		func(a *asset.ApprovedAssetInventory) { a.Scope.SourceSnapshotVersion++ },
		func(a *asset.ApprovedAssetInventory) { a.Assets[0].Role = "invalid" },
	} {
		a := inventoryFixture()
		mutate(a)
		_, err := Project(productFixture(), a, "shein")
		require.Error(t, err)
	}
	p := productFixture()
	p.Snapshot.Warnings = []catalog.Warning{{Message: strings.Repeat("x", MaxOutputBytes)}}
	_, err := Project(p, inventoryFixture(), "shein")
	require.Error(t, err)
}
