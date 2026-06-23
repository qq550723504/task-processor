package listingadmin

import (
	"context"
	"errors"
	"time"

	"task-processor/internal/infra/clients/management/api"
)

var ErrImportTaskNotFound = errors.New("import task not found")

type ImportTask struct {
	ID             int64      `json:"id"`
	TenantID       int64      `json:"tenantId"`
	StoreID        *int64     `json:"storeId,omitempty"`
	Platform       string     `json:"platform"`
	TargetPlatform string     `json:"targetPlatform,omitempty"`
	SourcePlatform string     `json:"sourcePlatform,omitempty"`
	Region         string     `json:"region"`
	CategoryID     *int64     `json:"categoryId,omitempty"`
	ProductID      string     `json:"productId"`
	Status         int16      `json:"status"`
	ProcessingNode string     `json:"processingNode,omitempty"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	ReasonCode     string     `json:"reasonCode,omitempty"`
	Stage          string     `json:"stage,omitempty"`
	RetryCount     int        `json:"retryCount"`
	MaxRetryCount  int        `json:"maxRetryCount"`
	Remark         string     `json:"remark,omitempty"`
	Priority       int        `json:"priority"`
	CreateTime     *time.Time `json:"createTime,omitempty"`
	UpdateTime     *time.Time `json:"updateTime,omitempty"`
}

type ImportTaskQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	StoreID     *int64
	Platform    string
	Region      string
	CategoryID  *int64
	ProductID   string
	Status      *int16
}

type ImportTaskPage struct {
	Items    []ImportTask `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type DispatchCandidateRequest struct {
	Platform         string
	Limit            int
	PerStoreLimit    int
	ExcludedStoreIDs []int64
}

type DispatchClaim struct {
	TaskID         int64
	PreviousStatus int16
	ProcessingNode string
	Remark         string
}

type ImportTaskRepository interface {
	ListImportTasks(ctx context.Context, query ImportTaskQuery) (*ImportTaskPage, error)
	BatchCreateImportTasks(ctx context.Context, tasks []ImportTask) ([]ImportTask, error)
	GetImportTaskByID(ctx context.Context, id int64) (*ImportTask, error)
	ListPendingAndRetryTasks(ctx context.Context, limit int, tenantID int64, storeIDs []int64) ([]ImportTask, error)
	ListDispatchCandidatesFair(ctx context.Context, req DispatchCandidateRequest) ([]ImportTask, error)
	ClaimForDispatch(ctx context.Context, claim DispatchClaim) (bool, error)
	RollbackDispatch(ctx context.Context, taskID int64, previousStatus int16, processingNode, reason string) error
	CountQueuedByStore(ctx context.Context, platform string, storeIDs []int64) (map[int64]int64, error)
	CountTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time) (int64, error)
	ListTimedOutProcessingTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]ImportTask, error)
	RecoverTimedOutProcessingTasks(ctx context.Context, ids []int64, recovery ProcessingTimeoutRecovery) (int, error)
	CountStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time) (int64, error)
	ListStaleQueuedTasks(ctx context.Context, timeoutBefore time.Time, limit int) ([]ImportTask, error)
	RecoverStaleQueuedTasks(ctx context.Context, ids []int64, recovery StaleQueuedRecovery) (int, error)
	UpdateImportTaskStatus(ctx context.Context, req *api.ProductImportTaskUpdateReqDTO) (bool, error)
	DeleteImportTask(ctx context.Context, tenantID, id int64) error
}

type ProcessingTimeoutRecovery struct {
	TimeoutMinutes int
	ErrorMessage   string
	ReasonCode     string
	Stage          string
	Remark         string
}

type StaleQueuedRecovery struct {
	TimeoutMinutes int
	ErrorMessage   string
	ReasonCode     string
	Stage          string
	Remark         string
}

type listingProductImportTask struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID       int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID    string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID        int64      `gorm:"column:store_id;not null;index"`
	Platform       string     `gorm:"column:platform;not null"`
	TargetPlatform string     `gorm:"column:target_platform"`
	SourcePlatform string     `gorm:"column:source_platform"`
	Region         string     `gorm:"column:region;not null"`
	CategoryID     int64      `gorm:"column:category_id;not null;index"`
	ProductID      string     `gorm:"column:product_id;not null;index"`
	Status         int16      `gorm:"column:status;not null;default:0;index"`
	ProcessingNode string     `gorm:"column:processing_node"`
	ErrorMessage   string     `gorm:"column:error_message"`
	ReasonCode     string     `gorm:"column:reason_code"`
	Stage          string     `gorm:"column:stage"`
	RetryCount     int        `gorm:"column:retry_count;not null;default:0"`
	MaxRetryCount  int        `gorm:"column:max_retry_count;not null;default:3"`
	Remark         string     `gorm:"column:remark"`
	Priority       int        `gorm:"column:priority;not null;default:5"`
	Creator        string     `gorm:"column:creator"`
	CreatedBy      string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime     *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater        string     `gorm:"column:updater"`
	UpdatedBy      string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime     *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted        int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingProductImportTask) TableName() string {
	return "listing_product_import_task"
}
