package sheinsync

import "time"

type SheinSyncTriggerMode string

const (
	SheinSyncTriggerModeManual   SheinSyncTriggerMode = "manual"
	SheinSyncTriggerModeSchedule SheinSyncTriggerMode = "schedule"
)

type SheinSyncJobStatus string

const (
	SheinSyncJobStatusPending            SheinSyncJobStatus = "pending"
	SheinSyncJobStatusRunning            SheinSyncJobStatus = "running"
	SheinSyncJobStatusSucceeded          SheinSyncJobStatus = "succeeded"
	SheinSyncJobStatusPartiallySucceeded SheinSyncJobStatus = "partially_succeeded"
	SheinSyncJobStatusFailed             SheinSyncJobStatus = "failed"
)

type SheinSyncJobRecord struct {
	ID               int64                `json:"id" gorm:"primaryKey"`
	TenantID         int64                `json:"tenant_id" gorm:"index:idx_listingkit_shein_sync_jobs_scope,priority:1"`
	StoreID          int64                `json:"store_id" gorm:"index:idx_listingkit_shein_sync_jobs_scope,priority:2"`
	TriggerMode      SheinSyncTriggerMode `json:"trigger_mode" gorm:"type:varchar(32);index;not null"`
	Status           SheinSyncJobStatus   `json:"status" gorm:"type:varchar(32);index;not null"`
	StartedAt        *time.Time           `json:"started_at,omitempty"`
	FinishedAt       *time.Time           `json:"finished_at,omitempty"`
	FetchedCount     int                  `json:"fetched_count"`
	InsertedCount    int                  `json:"inserted_count"`
	UpdatedCount     int                  `json:"updated_count"`
	DeactivatedCount int                  `json:"deactivated_count"`
	SkippedCount     int                  `json:"skipped_count"`
	ErrorSummary     string               `json:"error_summary,omitempty" gorm:"type:text"`
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
}

func (SheinSyncJobRecord) TableName() string {
	return "listingkit_shein_sync_jobs"
}
