package store

import "time"

type runRecord struct {
	TenantID           string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_runs_owner_idempotency,priority:1"`
	UserID             string `gorm:"column:owner_user_id;primaryKey;type:varchar(128);index;uniqueIndex:idx_image_agent_v2_runs_owner_idempotency,priority:2"`
	ID                 string `gorm:"primaryKey;type:varchar(64)"`
	BusinessTaskID     string `gorm:"type:varchar(64);index"`
	Mode               string `gorm:"type:varchar(32);not null"`
	IdempotencyKey     string `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_v2_runs_owner_idempotency,priority:3"`
	Status             string `gorm:"type:varchar(32);index;not null"`
	CurrentNode        string `gorm:"type:varchar(128)"`
	ActivePlanRevision int64  `gorm:"not null;default:0"`
	Version            int64  `gorm:"not null;default:0"`
	BudgetJSON         []byte
	UsageJSON          []byte
	BlockJSON          []byte
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (runRecord) TableName() string { return "image_agent_v2_runs" }

type planRecord struct {
	TenantID          string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_plans_run_idempotency,priority:1"`
	OwnerUserID       string `gorm:"primaryKey;type:varchar(128);uniqueIndex:idx_image_agent_v2_plans_run_idempotency,priority:2"`
	RunID             string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_plans_run_idempotency,priority:3"`
	Revision          int64  `gorm:"primaryKey"`
	ParentRevision    int64  `gorm:"not null;default:0"`
	IdempotencyKey    string `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_v2_plans_run_idempotency,priority:4"`
	SourceAssetIDs    []byte
	StyleReferenceIDs []byte
	CreatedBy         string    `gorm:"type:varchar(128)"`
	CreatedAt         time.Time `gorm:"not null"`
}

func (planRecord) TableName() string { return "image_agent_v2_plans" }

type slotRecord struct {
	TenantID          string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_slots_plan_idempotency,priority:1"`
	OwnerUserID       string `gorm:"primaryKey;type:varchar(128);uniqueIndex:idx_image_agent_v2_slots_plan_idempotency,priority:2"`
	RunID             string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_slots_plan_idempotency,priority:3"`
	PlanRevision      int64  `gorm:"primaryKey;uniqueIndex:idx_image_agent_v2_slots_plan_idempotency,priority:4"`
	ID                string `gorm:"primaryKey;type:varchar(64)"`
	Role              string `gorm:"type:varchar(32);index;not null"`
	SourceAssetIDs    []byte
	StyleReferenceIDs []byte
	Brief             string `gorm:"type:text"`
	IdempotencyKey    string `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_v2_slots_plan_idempotency,priority:5"`
	Status            string `gorm:"type:varchar(32);index;not null"`
	Attempt           int    `gorm:"not null;default:0"`
	CandidateAssetIDs []byte
	ErrorCode         string    `gorm:"type:varchar(128)"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

func (slotRecord) TableName() string { return "image_agent_v2_slots" }

type attemptRecord struct {
	TenantID       string    `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_attempts_slot_idempotency,priority:1"`
	OwnerUserID    string    `gorm:"primaryKey;type:varchar(128);uniqueIndex:idx_image_agent_v2_attempts_slot_idempotency,priority:2"`
	RunID          string    `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_attempts_slot_idempotency,priority:3"`
	PlanRevision   int64     `gorm:"primaryKey;uniqueIndex:idx_image_agent_v2_attempts_slot_idempotency,priority:4"`
	SlotID         string    `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_v2_attempts_slot_idempotency,priority:5"`
	Attempt        int       `gorm:"primaryKey"`
	Node           string    `gorm:"type:varchar(128);not null"`
	IdempotencyKey string    `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_v2_attempts_slot_idempotency,priority:6"`
	Outcome        string    `gorm:"type:varchar(64);not null"`
	ErrorCategory  string    `gorm:"type:varchar(128)"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (attemptRecord) TableName() string { return "image_agent_v2_attempts" }

type eventRecord struct {
	TenantID          string    `gorm:"primaryKey;type:varchar(64)"`
	OwnerUserID       string    `gorm:"primaryKey;type:varchar(128)"`
	RunID             string    `gorm:"primaryKey;type:varchar(64)"`
	Cursor            int64     `gorm:"primaryKey"`
	Type              string    `gorm:"type:varchar(64);index;not null"`
	ProjectionVersion int64     `gorm:"not null"`
	Payload           []byte    `gorm:"not null"`
	CreatedAt         time.Time `gorm:"not null"`
}

func (eventRecord) TableName() string { return "image_agent_v2_events" }

type assetCatalogRecord struct {
	TenantID     string `gorm:"primaryKey;type:varchar(64)"`
	OwnerUserID  string `gorm:"primaryKey;type:varchar(128)"`
	RunID        string `gorm:"primaryKey;type:varchar(64)"`
	ID           string `gorm:"primaryKey;type:varchar(128)"`
	Type         string `gorm:"type:varchar(16);not null"`
	URL          string `gorm:"type:text"`
	SourceURL    string `gorm:"type:text"`
	DisplayURL   string `gorm:"type:text"`
	Label        string `gorm:"type:varchar(256)"`
	Width        int
	Height       int
	MetadataJSON []byte
	CreatedAt    time.Time `gorm:"not null"`
}

func (assetCatalogRecord) TableName() string { return "image_agent_v2_asset_catalog" }

type assetCatalogManifestRecord struct {
	TenantID    string    `gorm:"primaryKey;type:varchar(64)"`
	OwnerUserID string    `gorm:"primaryKey;type:varchar(128)"`
	RunID       string    `gorm:"primaryKey;type:varchar(64)"`
	Version     int64     `gorm:"not null"`
	Hash        string    `gorm:"type:varchar(128);not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (assetCatalogManifestRecord) TableName() string { return "image_agent_v2_asset_catalog_manifests" }

type projectionRecord struct {
	TenantID     string    `gorm:"primaryKey;type:varchar(64)"`
	OwnerUserID  string    `gorm:"primaryKey;type:varchar(128)"`
	RunID        string    `gorm:"primaryKey;type:varchar(64)"`
	Version      int64     `gorm:"not null"`
	SnapshotJSON []byte    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (projectionRecord) TableName() string { return "image_agent_v2_projection_snapshots" }

type projectionCommitRecord struct {
	TenantID     string    `gorm:"primaryKey;type:varchar(64)"`
	OwnerUserID  string    `gorm:"primaryKey;type:varchar(128)"`
	RunID        string    `gorm:"primaryKey;type:varchar(64)"`
	CommitID     string    `gorm:"primaryKey;type:varchar(192)"`
	Fingerprint  string    `gorm:"type:varchar(64);not null"`
	Version      int64     `gorm:"not null"`
	SnapshotJSON []byte    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (projectionCommitRecord) TableName() string { return "image_agent_v2_projection_commits" }
