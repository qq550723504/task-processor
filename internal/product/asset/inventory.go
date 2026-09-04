package asset

type InventoryScope struct {
	TenantID              string `json:"tenant_id"`
	ProductKey            string `json:"product_key"`
	TargetPlatform        string `json:"target_platform,omitempty"`
	SourceSnapshotVersion uint64 `json:"source_snapshot_version,omitempty"`
}

// ApprovedAssetInventory contains only assets produced by committed human
// approval actions. Source candidates and staged outputs do not belong here.
type ApprovedAssetInventory struct {
	Scope  InventoryScope  `json:"scope"`
	Assets []ApprovedAsset `json:"assets"`
}
