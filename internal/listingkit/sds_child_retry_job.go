package listingkit

import (
	"context"
	"time"
)

type SDSChildRetryKind string

const SDSChildRetryKindDesignSync SDSChildRetryKind = "sds_design_sync"

type SDSChildRetryJobStatus string

const (
	SDSChildRetryJobStatusPending   SDSChildRetryJobStatus = "pending"
	SDSChildRetryJobStatusCompleted SDSChildRetryJobStatus = "completed"
	SDSChildRetryJobStatusExhausted SDSChildRetryJobStatus = "exhausted"
)

// SDSChildRetryJob is durable retry state for a single ListingKit child task.
// It intentionally does not use the parent task recovery flow, which reruns the
// complete ListingKit workflow.
type SDSChildRetryJob struct {
	ID          string                 `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TenantID    string                 `json:"tenant_id" gorm:"type:varchar(64);index"`
	TaskID      string                 `json:"task_id" gorm:"column:listingkit_task_id;type:varchar(96);uniqueIndex:uk_listingkit_sds_child_retry_task_kind,priority:1"`
	StoreID     int64                  `json:"store_id" gorm:"index"`
	Kind        SDSChildRetryKind      `json:"kind" gorm:"type:varchar(64);uniqueIndex:uk_listingkit_sds_child_retry_task_kind,priority:2"`
	Attempt     int                    `json:"attempt"`
	NextRetryAt time.Time              `json:"next_retry_at" gorm:"index"`
	ReasonCode  string                 `json:"reason_code" gorm:"type:varchar(96)"`
	LastError   string                 `json:"last_error" gorm:"type:text"`
	Status      SDSChildRetryJobStatus `json:"status" gorm:"type:varchar(32);index"`
	LeaseOwner  string                 `json:"lease_owner" gorm:"type:varchar(64);index"`
	LeaseUntil  *time.Time             `json:"lease_until" gorm:"index"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func (SDSChildRetryJob) TableName() string { return "listingkit_sds_child_retry_jobs" }

type SDSChildRetryJobRepository interface {
	ScheduleSDSChildRetry(ctx context.Context, job *SDSChildRetryJob) (*SDSChildRetryJob, error)
	ListDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int) ([]SDSChildRetryJob, error)
	ClaimDueSDSChildRetries(ctx context.Context, dueBefore time.Time, limit int, owner string, leaseUntil time.Time) ([]SDSChildRetryJob, error)
	SaveSDSChildRetry(ctx context.Context, job *SDSChildRetryJob) error
}

// StudioBatchSDSChildRetryResult records the tasks accepted by an explicit
// Studio batch retry request. Each accepted task is retried by the same durable
// worker used for automatically classified OSS failures.
type StudioBatchSDSChildRetryResult struct {
	BatchID   string                         `json:"batch_id"`
	Scheduled int                            `json:"scheduled"`
	Skipped   int                            `json:"skipped"`
	Failures  []StudioBatchSDSChildRetryFail `json:"failures,omitempty"`
}

type StudioBatchSDSChildRetryFail struct {
	TaskID  string `json:"task_id,omitempty"`
	Message string `json:"message"`
}
