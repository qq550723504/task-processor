package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrGenerationTopicPolicyNotFound = errors.New("generation topic policy not found")

type GenerationTopicPolicy struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenantId"`
	Platform   string     `json:"platform"`
	TopicKey   string     `json:"topicKey"`
	Remark     string     `json:"remark,omitempty"`
	Status     int16      `json:"status"`
	CreateTime *time.Time `json:"createTime,omitempty"`
	UpdateTime *time.Time `json:"updateTime,omitempty"`
}

type GenerationTopicPolicyQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Platform    string
	TopicKey    string
	Status      *int16
	Remark      string
}

type GenerationTopicPolicyPage struct {
	Items    []GenerationTopicPolicy `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

type GenerationTopicPolicyRepository interface {
	ListGenerationTopicPolicies(ctx context.Context, query GenerationTopicPolicyQuery) (*GenerationTopicPolicyPage, error)
	ListEnabledTopicKeys(ctx context.Context, tenantID int64, platform string) ([]string, error)
	GetGenerationTopicPolicy(ctx context.Context, tenantID, id int64) (*GenerationTopicPolicy, error)
	CreateGenerationTopicPolicy(ctx context.Context, policy *GenerationTopicPolicy) (*GenerationTopicPolicy, error)
	UpdateGenerationTopicPolicy(ctx context.Context, policy *GenerationTopicPolicy) (*GenerationTopicPolicy, error)
	UpdateGenerationTopicPolicyStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*GenerationTopicPolicy, error)
	DeleteGenerationTopicPolicy(ctx context.Context, tenantID, id int64) error
}

type listingGenerationTopicPolicy struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Platform    string     `gorm:"column:platform;type:varchar(32);not null;index"`
	TopicKey    string     `gorm:"column:topic_key;type:varchar(64);not null;index"`
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

func (listingGenerationTopicPolicy) TableName() string {
	return "listing_generation_topic_policy"
}

func (r listingGenerationTopicPolicy) toGenerationTopicPolicy() GenerationTopicPolicy {
	return GenerationTopicPolicy{
		ID:         r.ID,
		TenantID:   r.TenantID,
		Platform:   r.Platform,
		TopicKey:   r.TopicKey,
		Remark:     r.Remark,
		Status:     r.Status,
		CreateTime: r.CreateTime,
		UpdateTime: r.UpdateTime,
	}
}

func listingGenerationTopicPolicyFromGenerationTopicPolicy(policy *GenerationTopicPolicy) listingGenerationTopicPolicy {
	if policy == nil {
		return listingGenerationTopicPolicy{}
	}
	return listingGenerationTopicPolicy{
		ID:       policy.ID,
		TenantID: policy.TenantID,
		Platform: strings.TrimSpace(policy.Platform),
		TopicKey: strings.TrimSpace(policy.TopicKey),
		Remark:   strings.TrimSpace(policy.Remark),
		Status:   policy.Status,
	}
}

func applyGenerationTopicPolicyAuditFields(row *listingGenerationTopicPolicy, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}

func findGenerationTopicPolicyRows(ctx context.Context, db *gorm.DB, query GenerationTopicPolicyQuery) ([]listingGenerationTopicPolicy, int64, int, int, error) {
	var rows []listingGenerationTopicPolicy
	total, page, pageSize, err := findPagedRows(applyGenerationTopicPolicyQuery(db, query), query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return rows, total, page, pageSize, nil
}

func applyGenerationTopicPolicyQuery(db *gorm.DB, query GenerationTopicPolicyQuery) *gorm.DB {
	db = db.Where("deleted = 0")
	if query.TenantID > 0 {
		db = db.Where("tenant_id = ?", query.TenantID)
	}
	if platform := strings.TrimSpace(query.Platform); platform != "" {
		db = db.Where("platform = ?", platform)
	}
	if topicKey := strings.TrimSpace(query.TopicKey); topicKey != "" {
		db = db.Where("topic_key LIKE ?", "%"+topicKey+"%")
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if remark := strings.TrimSpace(query.Remark); remark != "" {
		db = db.Where("remark LIKE ?", "%"+remark+"%")
	}
	return db
}
