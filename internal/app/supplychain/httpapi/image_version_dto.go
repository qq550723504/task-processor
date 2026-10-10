package supplychainhttp

import (
	"strconv"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/asset"
)

// Catalog versions remain integers inside the owners and decimal strings on
// browser boundaries, including reads of the existing approved inventory.
type supplyImageSourceBody struct {
	asset.SourceSelectionRequest
	OriginalSnapshotVersion string `json:"originalSnapshotVersion"`
	EffectiveCatalogVersion string `json:"effectiveCatalogVersion"`
}

func (b supplyImageSourceBody) domain() (asset.SourceSelectionRequest, error) {
	result := b.SourceSelectionRequest
	for _, field := range []struct {
		text  string
		value *uint64
	}{{b.OriginalSnapshotVersion, &result.OriginalSnapshotVersion}, {b.EffectiveCatalogVersion, &result.EffectiveCatalogVersion}} {
		version, err := strconv.ParseUint(field.text, 10, 63)
		if err != nil || version == 0 || strconv.FormatUint(version, 10) != field.text {
			return asset.SourceSelectionRequest{}, preparation.ErrInvalid
		}
		*field.value = version
	}
	return result, nil
}

type supplyImageApprovalBody struct {
	asset.SourceApprovalCommand
	Selection supplyImageSourceBody `json:"selection"`
}

type supplyImageInventoryScope struct {
	asset.InventoryScope
	SourceSnapshotVersion string `json:"source_snapshot_version"`
}

type supplyImageInventoryResponse struct {
	asset.ApprovedAssetInventory
	Scope supplyImageInventoryScope `json:"scope"`
}

func supplyImageInventory(inventory asset.ApprovedAssetInventory) supplyImageInventoryResponse {
	return supplyImageInventoryResponse{ApprovedAssetInventory: inventory, Scope: supplyImageInventoryScope{InventoryScope: inventory.Scope, SourceSnapshotVersion: strconv.FormatUint(inventory.Scope.SourceSnapshotVersion, 10)}}
}
