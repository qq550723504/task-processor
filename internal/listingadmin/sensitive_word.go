package listingadmin

import (
	"context"
	"errors"
	"time"
)

var ErrSensitiveWordNotFound = errors.New("sensitive word not found")

type SensitiveWord struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenantId"`
	Word        string     `json:"word"`
	Language    string     `json:"language"`
	Tags        string     `json:"tags,omitempty"`
	Level       int        `json:"level"`
	ReplaceWord string     `json:"replaceWord,omitempty"`
	Remark      string     `json:"remark,omitempty"`
	Status      int16      `json:"status"`
	CreateTime  *time.Time `json:"createTime,omitempty"`
	UpdateTime  *time.Time `json:"updateTime,omitempty"`
}

type SensitiveWordQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Word        string
	Language    string
	Tags        string
	Level       *int
	Status      *int16
	Remark      string
}

type SensitiveWordPage struct {
	Items    []SensitiveWord `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type SensitiveWordRepository interface {
	ListSensitiveWords(ctx context.Context, query SensitiveWordQuery) (*SensitiveWordPage, error)
	GetSensitiveWord(ctx context.Context, tenantID, id int64) (*SensitiveWord, error)
	CreateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error)
	UpdateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error)
	UpdateSensitiveWordStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*SensitiveWord, error)
	DeleteSensitiveWord(ctx context.Context, tenantID, id int64) error
}

type listingSensitiveWord struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Word        string     `gorm:"column:word;not null;index"`
	Language    string     `gorm:"column:language;not null;index"`
	Tags        string     `gorm:"column:tags"`
	Level       int        `gorm:"column:level;not null;default:1;index"`
	ReplaceWord string     `gorm:"column:replace_word"`
	Remark      string     `gorm:"column:remark"`
	Status      int16      `gorm:"column:status;not null;default:0;index"`
	Creator     string     `gorm:"column:creator"`
	CreatedBy   string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater     string     `gorm:"column:updater"`
	UpdatedBy   string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted     int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingSensitiveWord) TableName() string {
	return "listing_sensitive_word"
}
