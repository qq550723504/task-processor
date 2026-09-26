package assetinspect

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

// These tests exercise already-approved domain binding/projection behavior;
// they make no choice about the public Tool input's platform selection.
func projectionFixture() (catalog.PublishedSnapshot, asset.ApprovedAssetInventory) {
	return catalog.PublishedSnapshot{
			Identity: catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"},
			Version:  9007199254740993, PublicationID: "publication-1",
			Snapshot: catalog.ProductSnapshot{Title: "Bottle"},
		}, asset.ApprovedAssetInventory{
			Scope:  asset.InventoryScope{TenantID: "org-a", ProductKey: "product-1", TargetPlatform: "fixture-platform", SourceSnapshotVersion: 9007199254740993},
			Assets: []asset.ApprovedAsset{{ID: "asset-1", RunID: "run-1", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: asset.RoleMain, URL: "https://assets.example/image.png"}},
		}
}

func TestProjectExactBindingAndNoInventedRevision(t *testing.T) {
	published, inventory := projectionFixture()
	raw, err := project(published, inventory, inventory.Scope)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatal(err)
	}
	if output["catalog_version"] != "9007199254740993" || output["product_key"] != "product-1" {
		t.Fatalf("exact product binding: %s", raw)
	}
	binding, ok := output["asset_binding"].(map[string]any)
	if !ok || binding["source_snapshot_version"] != "9007199254740993" || binding["target_platform"] != "fixture-platform" {
		t.Fatalf("asset binding: %s", raw)
	}
	for _, forbidden := range []string{"tenant_id", "inventory_revision", "approval_action_id", "approval_receipt", "task_id"} {
		if strings.Contains(string(raw), `"`+forbidden+`"`) {
			t.Fatalf("invented/authority field %s: %s", forbidden, raw)
		}
	}
	if !strings.Contains(string(raw), `"id":"asset-1"`) || !strings.Contains(string(raw), `"title":"Bottle"`) {
		t.Fatalf("missing exact facts: %s", raw)
	}
}

func TestProjectFailsClosedOnCorruptOrMissingFacts(t *testing.T) {
	for _, name := range []string{"organization", "product", "version", "platform", "publication", "empty", "invalid-role", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			published, inventory := projectionFixture()
			expected := inventory.Scope
			switch name {
			case "organization":
				inventory.Scope.TenantID = "org-b"
			case "product":
				inventory.Scope.ProductKey = "other"
			case "version":
				inventory.Scope.SourceSnapshotVersion--
			case "platform":
				inventory.Scope.TargetPlatform = "other"
			case "publication":
				published.PublicationID = ""
			case "empty":
				inventory.Assets = nil
			case "invalid-role":
				inventory.Assets[0].Role = "invented"
			case "duplicate":
				inventory.Assets = append(inventory.Assets, inventory.Assets[0])
			}
			raw, err := project(published, inventory, expected)
			if err == nil || len(raw) != 0 {
				t.Fatalf("corrupt facts returned: %s, %v", raw, err)
			}
		})
	}
}

func TestProjectBoundsCombinedProductAndAssets(t *testing.T) {
	published, inventory := projectionFixture()
	published.Snapshot.Description = strings.Repeat("x", MaxOutputBytes/2)
	inventory.Assets[0].URL = "https://assets.example/" + strings.Repeat("x", MaxOutputBytes/2)
	raw, err := project(published, inventory, inventory.Scope)
	if !errors.Is(err, errProjectionTooLarge) || len(raw) != 0 {
		t.Fatalf("combined oversize: %d bytes, %v", len(raw), err)
	}
}
