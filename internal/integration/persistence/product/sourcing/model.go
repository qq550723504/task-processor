package sourcingpersistence

import "time"

type publicationRecord struct {
	OrganizationID       string    `gorm:"primaryKey;size:128"`
	PublicationID        string    `gorm:"primaryKey;size:128"`
	InputHash            string    `gorm:"size:64;not null"`
	ProducerKind         string    `gorm:"size:128;not null"`
	ProducerVersion      string    `gorm:"size:128;not null"`
	ProductKey           string    `gorm:"size:128;not null"`
	ExpectedBaseVersion  *uint64   `gorm:"autoIncrement:false"`
	EnvelopeHash         string    `gorm:"size:64;not null"`
	SnapshotHash         string    `gorm:"size:64;not null"`
	EnvelopeJSON         []byte    `gorm:"type:bytea;not null;check:ck_source_publication_envelope_size,octet_length(envelope_json) BETWEEN 1 AND 2097152"`
	SnapshotJSON         []byte    `gorm:"type:bytea;not null;check:ck_source_publication_snapshot_size,octet_length(snapshot_json) BETWEEN 1 AND 2097152"`
	CatalogVersion       uint64    `gorm:"not null;autoIncrement:false;check:ck_source_publication_catalog_version,catalog_version > 0"`
	CatalogPublicationID string    `gorm:"size:128;not null"`
	ActorID              string    `gorm:"size:128;not null"`
	PublishedAt          time.Time `gorm:"not null"`
	EnvelopeBytes        int64     `gorm:"->;-:migration"`
	SnapshotBytes        int64     `gorm:"->;-:migration"`
}

func (publicationRecord) TableName() string { return "product_source_publications" }

type receiptRecord struct {
	OrganizationID       string    `gorm:"primaryKey;size:128"`
	PublicationID        string    `gorm:"primaryKey;size:128"`
	InputHash            string    `gorm:"size:64;not null"`
	ProducerKind         string    `gorm:"size:128;not null"`
	ProducerVersion      string    `gorm:"size:128;not null"`
	ProductKey           string    `gorm:"size:128;not null"`
	ExpectedBaseVersion  *uint64   `gorm:"autoIncrement:false"`
	CatalogVersion       uint64    `gorm:"not null;autoIncrement:false;check:ck_source_receipt_catalog_version,catalog_version > 0"`
	CatalogPublicationID string    `gorm:"size:128;not null"`
	EnvelopeHash         string    `gorm:"size:64;not null"`
	SnapshotHash         string    `gorm:"size:64;not null"`
	ActorID              string    `gorm:"size:128;not null"`
	PublishedAt          time.Time `gorm:"not null"`
}

func (receiptRecord) TableName() string { return "product_source_publication_receipts" }
