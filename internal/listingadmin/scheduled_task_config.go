package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrScheduledTaskConfigNotFound = errors.New("scheduled task config not found")

type ScheduledTaskConfig struct {
	ID              int64      `json:"id"`
	TenantID        int64      `json:"tenantId"`
	StoreID         int64      `json:"storeId"`
	Platform        string     `json:"platform"`
	TaskType        string     `json:"taskType"`
	Enabled         bool       `json:"enabled"`
	IntervalSeconds int        `json:"intervalSeconds"`
	Remark          string     `json:"remark,omitempty"`
	CreateTime      *time.Time `json:"createTime,omitempty"`
	UpdateTime      *time.Time `json:"updateTime,omitempty"`
}

type ScheduledTaskConfigQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	StoreID     *int64
	Platform    string
	TaskType    string
	Enabled     *bool
}

type ScheduledTaskConfigPage struct {
	Items    []ScheduledTaskConfig `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

type ScheduledTaskConfigRepository interface {
	ListScheduledTaskConfigs(ctx context.Context, query ScheduledTaskConfigQuery) (*ScheduledTaskConfigPage, error)
	GetScheduledTaskConfig(ctx context.Context, tenantID, id int64) (*ScheduledTaskConfig, error)
	UpsertScheduledTaskConfig(ctx context.Context, config *ScheduledTaskConfig) (*ScheduledTaskConfig, error)
	UpdateScheduledTaskConfigStatus(ctx context.Context, tenantID, id int64, enabled bool, remark string) (*ScheduledTaskConfig, error)
	DeleteScheduledTaskConfig(ctx context.Context, tenantID, id int64) error
	ListEnabledScheduledTaskConfigs(ctx context.Context, platform, taskType string) ([]ScheduledTaskConfig, error)
}

type listingScheduledTaskConfig struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID        int64      `gorm:"column:tenant_id;not null;index;uniqueIndex:uk_listing_scheduled_task_config_scope,priority:1"`
	OwnerUserID     string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	StoreID         int64      `gorm:"column:store_id;not null;index;uniqueIndex:uk_listing_scheduled_task_config_scope,priority:2"`
	Platform        string     `gorm:"column:platform;type:varchar(32);not null;index;uniqueIndex:uk_listing_scheduled_task_config_scope,priority:3"`
	TaskType        string     `gorm:"column:task_type;type:varchar(64);not null;index;uniqueIndex:uk_listing_scheduled_task_config_scope,priority:4"`
	Enabled         int16      `gorm:"column:enabled;not null;default:0;index"`
	IntervalSeconds int        `gorm:"column:interval_seconds;not null;default:3600"`
	Remark          string     `gorm:"column:remark;type:text"`
	Creator         string     `gorm:"column:creator"`
	CreatedBy       string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime      *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater         string     `gorm:"column:updater"`
	UpdatedBy       string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime      *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted         int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingScheduledTaskConfig) TableName() string { return "listing_scheduled_task_config" }
