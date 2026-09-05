package readiness

import (
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
)

// ProductInputReadiness describes only the canonical Product input gate.
// Ready does not establish marketplace category, payload or submission readiness.
type ProductInputReadiness struct {
	Ready          bool              `json:"ready"`
	Blockers       []string          `json:"blockers,omitempty"`
	SourceWarnings []catalog.Warning `json:"source_warnings,omitempty"`
}

// ProductInputs consumes a pinned Catalog snapshot and the matching inventory
// read from Product Asset. It never promotes snapshot images to approved assets.
func ProductInputs(product catalog.PublishedSnapshot, inventory *asset.ApprovedAssetInventory, targetPlatform string) ProductInputReadiness {
	result := ProductInputReadiness{SourceWarnings: append([]catalog.Warning(nil), product.Snapshot.Warnings...)}
	if product.Version == 0 || catalog.ValidateSnapshotIdentity(product.Identity) != nil {
		result.Blockers = append(result.Blockers, "product_snapshot_not_ready")
	}
	if product.Snapshot.Review != nil && product.Snapshot.Review.NeedsReview {
		result.Blockers = append(result.Blockers, "source_review_required")
	}
	if inventory == nil || len(inventory.Assets) == 0 || inventory.Scope.TenantID != product.Identity.TenantID || inventory.Scope.ProductKey != product.Identity.ProductKey || inventory.Scope.SourceSnapshotVersion != product.Version || inventory.Scope.TargetPlatform != targetPlatform {
		result.Blockers = append(result.Blockers, "approved_assets_not_ready")
	}
	result.Ready = len(result.Blockers) == 0
	return result
}
