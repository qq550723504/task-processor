package listingkit

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinworkspace "task-processor/internal/workspace/shein"
)

type Task struct {
	ID                           string                        `json:"id" gorm:"primaryKey;type:varchar(36)"`
	TenantID                     string                        `json:"tenant_id,omitempty" gorm:"type:varchar(64);index"`
	UserID                       string                        `json:"user_id,omitempty" gorm:"type:varchar(128);index"`
	Request                      *GenerateRequest              `json:"request" gorm:"type:text"`
	SheinStoreResolutionSnapshot *SheinStoreResolutionSnapshot `json:"shein_store_resolution_snapshot,omitempty" gorm:"type:text"`
	Status                       TaskStatus                    `json:"status" gorm:"type:varchar(20);index"`
	Result                       *ListingKitResult             `json:"result,omitempty" gorm:"type:text"`
	Error                        string                        `json:"error,omitempty" gorm:"type:text"`
	CreatedAt                    time.Time                     `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt                    time.Time                     `json:"updated_at" gorm:"autoUpdateTime"`
	RetryCount                   int                           `json:"retry_count" gorm:"default:0"`
}

type TaskResult struct {
	TaskIdentityFields
	TaskResultLifecycleFields
	SheinSubmissionStatusFields
	Result        *ListingKitResult `json:"result,omitempty"`
	ReviewReasons []string          `json:"review_reasons,omitempty"`
}

type TaskListQuery struct {
	TenantID            string `form:"tenant_id" json:"tenant_id,omitempty"`
	Status              string `form:"status" json:"status,omitempty"`
	Platform            string `form:"platform" json:"platform,omitempty"`
	SheinWorkflowStatus string `form:"shein_workflow_status" json:"shein_workflow_status,omitempty"`
	SheinBlockerKey     string `form:"shein_blocker_key" json:"shein_blocker_key,omitempty"`
	SheinWarningKey     string `form:"shein_warning_key" json:"shein_warning_key,omitempty"`
	SheinWorkQueue      string `form:"shein_work_queue" json:"shein_work_queue,omitempty"`
	SheinActionQueue    string `form:"shein_action_queue" json:"shein_action_queue,omitempty"`
	Page                int    `form:"page" json:"page,omitempty"`
	PageSize            int    `form:"page_size" json:"page_size,omitempty"`
}

type TaskListItem struct {
	TaskIdentityFields
	TaskListLifecycleFields
	TaskListDisplayFields
	SheinTaskListWorkflowFields
	SheinTaskListStoreFields
	SheinTaskListSubmissionFields
}

type TaskIdentityFields struct {
	TaskID   string `json:"task_id"`
	TenantID string `json:"tenant_id,omitempty"`
}

type TaskResultLifecycleFields struct {
	Status      TaskStatus `json:"status"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TaskListLifecycleFields struct {
	Status      TaskStatus `json:"status"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type TaskListDisplayFields struct {
	Platforms     []string `json:"platforms,omitempty"`
	Title         string   `json:"title,omitempty"`
	ImageCount    int      `json:"image_count"`
	ProductName   string   `json:"product_name,omitempty"`
	VariantLabel  string   `json:"variant_label,omitempty"`
	SDSSyncStatus string   `json:"sds_sync_status,omitempty"`
}

type SheinSubmissionStatusFields struct {
	SheinWorkflowStatus         string `json:"shein_workflow_status,omitempty"`
	SheinLatestSubmissionStatus string `json:"shein_latest_submission_status,omitempty"`
	SheinLatestSubmissionError  string `json:"shein_latest_submission_error,omitempty"`
	SheinSubmissionRemoteStatus string `json:"shein_submission_remote_status,omitempty"`
}

type SheinTaskListWorkflowFields struct {
	SheinSubmissionStatusFields
	SheinBlockingKeys   []string                       `json:"shein_blocking_keys,omitempty"`
	SheinWarningKeys    []string                       `json:"shein_warning_keys,omitempty"`
	SheinWorkQueue      string                         `json:"shein_work_queue,omitempty"`
	SheinActionQueue    string                         `json:"shein_action_queue,omitempty"`
	SheinStatusOverview *sheinworkspace.StatusOverview `json:"shein_status_overview,omitempty"`
}

type SheinTaskListStoreFields struct {
	SheinStoreID               int64      `json:"shein_store_id,omitempty"`
	SheinStoreSite             string     `json:"shein_store_site,omitempty"`
	SheinStoreProfileID        int64      `json:"shein_store_profile_id,omitempty"`
	SheinStoreResolvedAt       *time.Time `json:"shein_store_resolved_at,omitempty"`
	SheinStoreStrategy         string     `json:"shein_store_strategy,omitempty"`
	SheinStoreReason           string     `json:"shein_store_reason,omitempty"`
	SheinStoreMatchedRuleKinds []string   `json:"shein_store_matched_rule_kinds,omitempty"`
	SheinStoreManualOverride   bool       `json:"shein_store_manual_override,omitempty"`
	SheinStoreFallback         bool       `json:"shein_store_fallback,omitempty"`
}

type SheinTaskListSubmissionFields struct {
	SheinSubmissionRemoteCheckedAt *time.Time `json:"shein_submission_remote_checked_at,omitempty"`
	SheinSubmissionRemoteRecordID  string     `json:"shein_submission_remote_record_id,omitempty"`
}

type SheinStoreResolutionSnapshot struct {
	StoreID           int64                `json:"store_id,omitempty"`
	Site              string               `json:"site,omitempty"`
	WarehouseCode     string               `json:"warehouse_code,omitempty"`
	DefaultStock      int                  `json:"default_stock,omitempty"`
	DefaultSubmitMode string               `json:"default_submit_mode,omitempty"`
	Pricing           sheinpub.PricingRule `json:"pricing,omitempty"`
	Strategy          string               `json:"strategy,omitempty"`
	Reason            string               `json:"reason,omitempty"`
	MatchedRuleKinds  []string             `json:"matched_rule_kinds,omitempty"`
	MatchedProfileID  int64                `json:"matched_profile_id,omitempty"`
	ManualOverride    bool                 `json:"manual_override,omitempty"`
	Fallback          bool                 `json:"fallback,omitempty"`
	ResolvedAt        time.Time            `json:"resolved_at,omitempty"`
}

type TaskListSummary struct {
	StatusCounts              map[string]int `json:"status_counts,omitempty"`
	SheinWorkflowStatusCounts map[string]int `json:"shein_workflow_status_counts,omitempty"`
	SheinWorkQueueCounts      map[string]int `json:"shein_work_queue_counts,omitempty"`
	SheinActionQueueCounts    map[string]int `json:"shein_action_queue_counts,omitempty"`
	SheinBlockerCounts        map[string]int `json:"shein_blocker_counts,omitempty"`
	SheinWarningCounts        map[string]int `json:"shein_warning_counts,omitempty"`
}

type TaskFacetDescriptor struct {
	Key         string `json:"key"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
	Severity    string `json:"severity,omitempty"`
}

type TaskListTaxonomy struct {
	SheinWorkflowStatuses []TaskFacetDescriptor `json:"shein_workflow_statuses,omitempty"`
	SheinWorkQueues       []TaskFacetDescriptor `json:"shein_work_queues,omitempty"`
	SheinActionQueues     []TaskFacetDescriptor `json:"shein_action_queues,omitempty"`
	SheinBlockers         []TaskFacetDescriptor `json:"shein_blockers,omitempty"`
	SheinWarnings         []TaskFacetDescriptor `json:"shein_warnings,omitempty"`
}

type TaskListPage struct {
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int64             `json:"total"`
	Summary  *TaskListSummary  `json:"summary,omitempty"`
	Taxonomy *TaskListTaxonomy `json:"taxonomy,omitempty"`
	Items    []TaskListItem    `json:"items,omitempty"`
}

func (r SheinStoreResolutionSnapshot) Value() (driver.Value, error) { return json.Marshal(r) }

func (r *SheinStoreResolutionSnapshot) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, r)
}
