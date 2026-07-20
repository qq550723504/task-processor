package listingkit

import (
	"context"
	"time"
)

type SDSChildRetryKind string

const SDSChildRetryKindDesignSync SDSChildRetryKind = "sds_design_sync"

type SDSChildRetryJobStatus string

const SDSChildRetryJobStatusPending SDSChildRetryJobStatus = "pending"

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
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

func (SDSChildRetryJob) TableName() string { return "listingkit_sds_child_retry_jobs" }

type SDSChildRetryJobRepository interface {
	ScheduleSDSChildRetry(ctx context.Context, job *SDSChildRetryJob) (*SDSChildRetryJob, error)
}
