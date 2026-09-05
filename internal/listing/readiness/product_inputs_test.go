package readiness

import (
	"testing"

	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

func TestProductInputsRequireScopedApprovedAssetsAndExposeSourceReview(t *testing.T) {
	snapshot := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org-a", ProductKey: "product-1"}, Version: 1, Snapshot: catalog.ProductSnapshot{Title: "Bottle", Images: []catalog.Image{{URL: "https://source.test/first.jpg"}}}}
	if got := ProductInputs(snapshot, nil, "shein"); got.Ready || !hasCode(got, "approved_assets_not_ready") {
		t.Fatalf("source image implicitly approved: %+v", got)
	}
	inventory := asset.ApprovedAssetInventory{Scope: asset.InventoryScope{TenantID: "org-a", ProductKey: "product-1", SourceSnapshotVersion: 1, TargetPlatform: "shein"}, Assets: []asset.ApprovedAsset{{ID: "approved-1", Role: asset.RoleMain, URL: "https://approved.test/main.jpg"}}}
	if got := ProductInputs(snapshot, &inventory, "shein"); !got.Ready {
		t.Fatalf("approved inputs not ready: %+v", got)
	}
	for _, scope := range []asset.InventoryScope{
		{TenantID: "org-b", ProductKey: "product-1", SourceSnapshotVersion: 1, TargetPlatform: "shein"},
		{TenantID: "org-a", ProductKey: "product-2", SourceSnapshotVersion: 1, TargetPlatform: "shein"},
		{TenantID: "org-a", ProductKey: "product-1", SourceSnapshotVersion: 2, TargetPlatform: "shein"},
		{TenantID: "org-a", ProductKey: "product-1", SourceSnapshotVersion: 1, TargetPlatform: "amazon"},
	} {
		inventory.Scope = scope
		if got := ProductInputs(snapshot, &inventory, "shein"); got.Ready {
			t.Fatal("mismatched inventory accepted")
		}
	}
	snapshot.Snapshot.Review = &catalog.ReviewState{NeedsReview: true, Reasons: []string{"missing source dimensions"}}
	snapshot.Snapshot.Warnings = []catalog.Warning{{Code: "missing_dimensions", Field: "dimensions", Message: "missing source dimensions"}}
	got := ProductInputs(snapshot, nil, "shein")
	if got.Ready || !hasCode(got, "source_review_required") || !hasCode(got, "approved_assets_not_ready") || len(got.SourceWarnings) != 1 {
		t.Fatalf("missing facts hidden: %+v", got)
	}
	if got := ProductInputs(catalog.PublishedSnapshot{}, nil, "shein"); got.Ready || !hasCode(got, "product_snapshot_not_ready") {
		t.Fatal("empty product accepted")
	}
}
func hasCode(result ProductInputReadiness, code string) bool {
	for _, finding := range result.Blockers {
		if finding == code {
			return true
		}
	}
	return false
}
