package listingkit

import "time"

type Task struct {
	ID         string            `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID   string            `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	Request    *GenerateRequest  `json:"request" gorm:"type:text"`
	Status     TaskStatus        `json:"status" gorm:"type:varchar(20);index"`
	Result     *ListingKitResult `json:"result,omitempty" gorm:"type:text"`
	Error      string            `json:"error,omitempty" gorm:"type:text"`
	CreatedAt  time.Time         `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time         `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount int               `json:"retry_count" gorm:"default:0"`
}

type TaskResult struct {
	TaskID        string            `json:"task_id"`
	TenantID      string            `json:"tenant_id,omitempty"`
	Status        TaskStatus        `json:"status"`
	Result        *ListingKitResult `json:"result,omitempty"`
	Error         string            `json:"error,omitempty"`
	ReviewReasons []string          `json:"review_reasons,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
}

type TaskListQuery struct {
	TenantID            string `form:"tenant_id" json:"tenant_id,omitempty"`
	Status              string `form:"status" json:"status,omitempty"`
	Platform            string `form:"platform" json:"platform,omitempty"`
	SheinWorkflowStatus string `form:"shein_workflow_status" json:"shein_workflow_status,omitempty"`
	Page                int    `form:"page" json:"page,omitempty"`
	PageSize            int    `form:"page_size" json:"page_size,omitempty"`
}

type TaskListItem struct {
	TaskID                         string     `json:"task_id"`
	TenantID                       string     `json:"tenant_id,omitempty"`
	Status                         TaskStatus `json:"status"`
	Platforms                      []string   `json:"platforms,omitempty"`
	Title                          string     `json:"title,omitempty"`
	ImageCount                     int        `json:"image_count"`
	ProductName                    string     `json:"product_name,omitempty"`
	VariantLabel                   string     `json:"variant_label,omitempty"`
	SDSSyncStatus                  string     `json:"sds_sync_status,omitempty"`
	SheinWorkflowStatus            string     `json:"shein_workflow_status,omitempty"`
	SheinLatestSubmissionStatus    string     `json:"shein_latest_submission_status,omitempty"`
	SheinLatestSubmissionError     string     `json:"shein_latest_submission_error,omitempty"`
	SheinSubmissionRemoteStatus    string     `json:"shein_submission_remote_status,omitempty"`
	SheinSubmissionRemoteCheckedAt *time.Time `json:"shein_submission_remote_checked_at,omitempty"`
	SheinSubmissionRemoteRecordID  string     `json:"shein_submission_remote_record_id,omitempty"`
	Error                          string     `json:"error,omitempty"`
	CreatedAt                      time.Time  `json:"created_at"`
	UpdatedAt                      time.Time  `json:"updated_at"`
	CompletedAt                    *time.Time `json:"completed_at,omitempty"`
}

type TaskListPage struct {
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
	Items    []TaskListItem `json:"items,omitempty"`
}
