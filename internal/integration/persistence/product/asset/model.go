package assetpersistence

// ApprovedAssetRecord persists one approved product asset. The composite
// primary key preserves the ImageAgent approval identity, while AssetID is
// unique only inside a tenant.
type ApprovedAssetRecord struct {
	TenantID     string `gorm:"primaryKey;size:128;uniqueIndex:ux_product_approved_asset_id,priority:1;index:ix_product_approved_inventory,priority:1"`
	RunID        string `gorm:"primaryKey;size:128"`
	PlanRevision int64  `gorm:"primaryKey;autoIncrement:false"`
	SlotID       string `gorm:"primaryKey;size:128"`
	Attempt      int    `gorm:"primaryKey;autoIncrement:false"`
	ActionID     string `gorm:"primaryKey;size:128"`
	AssetID      string `gorm:"size:128;not null;uniqueIndex:ux_product_approved_asset_id,priority:2"`
	ProductKey   string `gorm:"size:128;not null;index:ix_product_approved_inventory,priority:2"`
	PayloadJSON  []byte `gorm:"type:json;not null"`
}

func (ApprovedAssetRecord) TableName() string { return "product_approved_assets" }

// ApprovalReceiptRecord is the tenant-qualified idempotency receipt for an
// approval action.
type ApprovalReceiptRecord struct {
	TenantID     string `gorm:"primaryKey;size:128"`
	ActionID     string `gorm:"primaryKey;size:128"`
	PayloadHash  string `gorm:"size:64;not null"`
	AssetIDsJSON []byte `gorm:"type:json;not null"`
}

func (ApprovalReceiptRecord) TableName() string { return "product_approval_receipts" }

// ApprovedInventoryHeadRecord points at the one approval action whose full
// asset set is currently authoritative for a tenant-qualified product.
// Historical approved assets remain immutable and queryable for audit, while
// consumers never have to infer recency from opaque run or action IDs.
type ApprovedInventoryHeadRecord struct {
	TenantID   string `gorm:"primaryKey;size:128"`
	ProductKey string `gorm:"primaryKey;size:128"`
	ActionID   string `gorm:"size:128;not null"`
}

func (ApprovedInventoryHeadRecord) TableName() string { return "product_approved_inventory_heads" }
