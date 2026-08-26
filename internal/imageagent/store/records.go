package store

import "time"

type runRecord struct {
	TenantID           string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_runs_tenant_idempotency,priority:1"`
	ID                 string `gorm:"primaryKey;type:varchar(64)"`
	BusinessTaskID     string `gorm:"type:varchar(64);index"`
	UserID             string `gorm:"type:varchar(128);index"`
	Mode               string `gorm:"type:varchar(32);not null"`
	IdempotencyKey     string `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_runs_tenant_idempotency"`
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

func (runRecord) TableName() string { return "image_agent_runs" }

type planRecord struct {
	TenantID          string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_plans_run_idempotency,priority:1"`
	RunID             string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_plans_run_idempotency,priority:2"`
	Revision          int64  `gorm:"primaryKey"`
	ParentRevision    int64  `gorm:"not null;default:0"`
	IdempotencyKey    string `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_plans_run_idempotency,priority:3"`
	SourceAssetIDs    []byte
	StyleReferenceIDs []byte
	CreatedBy         string    `gorm:"type:varchar(128)"`
	CreatedAt         time.Time `gorm:"not null"`
}

func (planRecord) TableName() string { return "image_agent_plans" }

type slotRecord struct {
	TenantID          string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_slots_plan_idempotency,priority:1"`
	RunID             string `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_slots_plan_idempotency,priority:2"`
	PlanRevision      int64  `gorm:"primaryKey;uniqueIndex:idx_image_agent_slots_plan_idempotency,priority:3"`
	ID                string `gorm:"primaryKey;type:varchar(64)"`
	Role              string `gorm:"type:varchar(32);index;not null"`
	SourceAssetIDs    []byte
	StyleReferenceIDs []byte
	Brief             string `gorm:"type:text"`
	IdempotencyKey    string `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_slots_plan_idempotency,priority:4"`
	Status            string `gorm:"type:varchar(32);index;not null"`
	Attempt           int    `gorm:"not null;default:0"`
	CandidateAssetIDs []byte
	ErrorCode         string    `gorm:"type:varchar(128)"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

func (slotRecord) TableName() string { return "image_agent_slots" }

type attemptRecord struct {
	TenantID       string    `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_attempts_slot_idempotency,priority:1"`
	RunID          string    `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_attempts_slot_idempotency,priority:2"`
	SlotID         string    `gorm:"primaryKey;type:varchar(64);uniqueIndex:idx_image_agent_attempts_slot_idempotency,priority:3"`
	Attempt        int       `gorm:"primaryKey"`
	Node           string    `gorm:"type:varchar(128);not null"`
	IdempotencyKey string    `gorm:"type:varchar(128);not null;uniqueIndex:idx_image_agent_attempts_slot_idempotency,priority:4"`
	Outcome        string    `gorm:"type:varchar(64);not null"`
	ErrorCategory  string    `gorm:"type:varchar(128)"`
	CreatedAt      time.Time `gorm:"not null"`
}

func (attemptRecord) TableName() string { return "image_agent_attempts" }

type eventRecord struct {
	TenantID          string    `gorm:"primaryKey;type:varchar(64)"`
	RunID             string    `gorm:"primaryKey;type:varchar(64)"`
	Cursor            int64     `gorm:"primaryKey"`
	Type              string    `gorm:"type:varchar(64);index;not null"`
	ProjectionVersion int64     `gorm:"not null"`
	Payload           []byte    `gorm:"not null"`
	CreatedAt         time.Time `gorm:"not null"`
}

func (eventRecord) TableName() string { return "image_agent_events" }
