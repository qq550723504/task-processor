package sheinsync

import "time"

type SheinEnrollmentRunTriggerMode string

const (
	SheinEnrollmentRunTriggerModeManualConfirmed SheinEnrollmentRunTriggerMode = "manual_confirmed"
	SheinEnrollmentRunTriggerModeAutoSchedule    SheinEnrollmentRunTriggerMode = "auto_schedule"
)

type SheinEnrollmentRunStatus string

const (
	SheinEnrollmentRunStatusPending            SheinEnrollmentRunStatus = "pending"
	SheinEnrollmentRunStatusRunning            SheinEnrollmentRunStatus = "running"
	SheinEnrollmentRunStatusSucceeded          SheinEnrollmentRunStatus = "succeeded"
	SheinEnrollmentRunStatusPartiallySucceeded SheinEnrollmentRunStatus = "partially_succeeded"
	SheinEnrollmentRunStatusFailed             SheinEnrollmentRunStatus = "failed"
	SheinEnrollmentRunStatusCancelled          SheinEnrollmentRunStatus = "cancelled"
)

type SheinEnrollmentItemStatus string

const (
	SheinEnrollmentItemStatusPending   SheinEnrollmentItemStatus = "pending"
	SheinEnrollmentItemStatusRunning   SheinEnrollmentItemStatus = "running"
	SheinEnrollmentItemStatusSucceeded SheinEnrollmentItemStatus = "succeeded"
	SheinEnrollmentItemStatusFailed    SheinEnrollmentItemStatus = "failed"
	SheinEnrollmentItemStatusCancelled SheinEnrollmentItemStatus = "cancelled"
)

type SheinActivityEnrollmentRunRecord struct {
	ID             int64                         `json:"id" gorm:"primaryKey"`
	TenantID       int64                         `json:"tenant_id" gorm:"index:idx_listingkit_shein_enrollment_runs_scope,priority:1"`
	StoreID        int64                         `json:"store_id" gorm:"index:idx_listingkit_shein_enrollment_runs_scope,priority:2"`
	ActivityType   string                        `json:"activity_type" gorm:"type:varchar(64);index"`
	ActivityKey    string                        `json:"activity_key" gorm:"type:varchar(128);index"`
	TriggerMode    SheinEnrollmentRunTriggerMode `json:"trigger_mode" gorm:"type:varchar(32);index;not null"`
	Status         SheinEnrollmentRunStatus      `json:"status" gorm:"type:varchar(32);index;not null"`
	CandidateCount int                           `json:"candidate_count"`
	SubmittedCount int                           `json:"submitted_count"`
	SucceededCount int                           `json:"succeeded_count"`
	FailedCount    int                           `json:"failed_count"`
	StartedAt      *time.Time                    `json:"started_at,omitempty"`
	FinishedAt     *time.Time                    `json:"finished_at,omitempty"`
	ErrorSummary   string                        `json:"error_summary,omitempty" gorm:"type:text"`
	CreatedAt      time.Time                     `json:"created_at"`
	UpdatedAt      time.Time                     `json:"updated_at"`
}

func (SheinActivityEnrollmentRunRecord) TableName() string {
	return "listingkit_shein_activity_enrollment_runs"
}

type SheinActivityEnrollmentItemRecord struct {
	ID               int64                     `json:"id" gorm:"primaryKey"`
	RunID            int64                     `json:"run_id" gorm:"index:idx_listingkit_shein_enrollment_items_run_candidate,priority:1;uniqueIndex:uk_listingkit_shein_enrollment_items_run_candidate,priority:1"`
	CandidateID      int64                     `json:"candidate_id" gorm:"index:idx_listingkit_shein_enrollment_items_run_candidate,priority:2;uniqueIndex:uk_listingkit_shein_enrollment_items_run_candidate,priority:2"`
	StoreID          int64                     `json:"store_id" gorm:"index"`
	ActivityKey      string                    `json:"activity_key,omitempty" gorm:"type:varchar(128);index"`
	CandidateVersion string                    `json:"candidate_version,omitempty" gorm:"type:varchar(64);index"`
	SyncedProductID  int64                     `json:"synced_product_id" gorm:"index"`
	SKCName          string                    `json:"skc_name,omitempty" gorm:"type:varchar(128);index"`
	Status           SheinEnrollmentItemStatus `json:"status" gorm:"type:varchar(32);index;not null"`
	RequestPayload   string                    `json:"request_payload,omitempty" gorm:"type:text"`
	ResponsePayload  string                    `json:"response_payload,omitempty" gorm:"type:text"`
	ErrorMessage     string                    `json:"error_message,omitempty" gorm:"type:text"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

func (SheinActivityEnrollmentItemRecord) TableName() string {
	return "listingkit_shein_activity_enrollment_items"
}
