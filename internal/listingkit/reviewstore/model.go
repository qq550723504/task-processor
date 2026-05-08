package reviewstore

import (
	"time"

	"gorm.io/gorm"
)

type ReviewRecord struct {
	ID              uint      `gorm:"primaryKey" json:"-"`
	TenantID        string    `gorm:"index:idx_listingkit_review_lookup,priority:1;type:varchar(64)"`
	TaskID          string    `gorm:"index:idx_listingkit_review_lookup,priority:2;type:varchar(64);not null"`
	Platform        string    `gorm:"index:idx_listingkit_review_lookup,priority:3;type:varchar(32);not null"`
	Slot            string    `gorm:"index:idx_listingkit_review_lookup,priority:4;type:varchar(64);not null"`
	Capability      string    `gorm:"index:idx_listingkit_review_lookup,priority:5;type:varchar(64);not null"`
	Decision        string    `gorm:"type:varchar(32);not null"`
	Status          string    `gorm:"type:varchar(32);not null"`
	Message         string    `gorm:"type:text"`
	ReviewedAt      time.Time `gorm:"index;not null"`
	ReviewedBy      string    `gorm:"type:varchar(128)"`
	AssetID         string    `gorm:"type:varchar(128)"`
	AssetRevision   string    `gorm:"type:varchar(128)"`
	PreviewRevision string    `gorm:"type:varchar(128)"`
	TaskRevision    string    `gorm:"type:varchar(128)"`
	SourceActionKey string    `gorm:"type:varchar(64)"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}
