package catalogpersistence

// SnapshotVersionRecord stores one immutable version. PublicationID is unique
// inside the exact tenant/product stream and therefore serves as its replay key.
type SnapshotVersionRecord struct {
	TenantID      string `gorm:"primaryKey;size:128;uniqueIndex:ux_product_snapshot_publication,priority:1"`
	ProductKey    string `gorm:"primaryKey;size:128;uniqueIndex:ux_product_snapshot_publication,priority:2"`
	Version       uint64 `gorm:"primaryKey;autoIncrement:false"`
	PublicationID string `gorm:"size:128;not null;uniqueIndex:ux_product_snapshot_publication,priority:3"`
	PayloadHash   string `gorm:"size:64;not null"`
	SnapshotJSON  []byte `gorm:"type:json;not null"`
}

func (SnapshotVersionRecord) TableName() string { return "product_snapshot_versions" }

// SnapshotHeadRecord points to the current immutable version for one exact
// tenant/product identity.
type SnapshotHeadRecord struct {
	TenantID       string `gorm:"primaryKey;size:128"`
	ProductKey     string `gorm:"primaryKey;size:128"`
	CurrentVersion uint64 `gorm:"not null;autoIncrement:false"`
	PublicationID  string `gorm:"size:128;not null"`
}

func (SnapshotHeadRecord) TableName() string { return "product_snapshot_heads" }
