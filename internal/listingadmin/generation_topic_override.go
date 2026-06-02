package listingadmin

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrGenerationTopicOverrideNotFound = errors.New("generation topic override not found")

type GenerationTopicOverride struct {
	ID                          int64               `json:"id"`
	TenantID                    int64               `json:"tenantId"`
	Platform                    string              `json:"platform"`
	TopicKey                    string              `json:"topicKey"`
	AdditionalPromptDirectives  []string            `json:"additionalPromptDirectives,omitempty"`
	AdditionalLexiconByLanguage map[string][]string `json:"additionalLexiconByLanguage,omitempty"`
	Remark                      string              `json:"remark,omitempty"`
	Status                      int16               `json:"status"`
	CreateTime                  *time.Time          `json:"createTime,omitempty"`
	UpdateTime                  *time.Time          `json:"updateTime,omitempty"`
}

type GenerationTopicOverrideQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Platform    string
	TopicKey    string
	Status      *int16
	Remark      string
}

type GenerationTopicOverridePage struct {
	Items    []GenerationTopicOverride `json:"items"`
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
}

type GenerationTopicOverrideRepository interface {
	ListGenerationTopicOverrides(ctx context.Context, query GenerationTopicOverrideQuery) (*GenerationTopicOverridePage, error)
	GetGenerationTopicOverride(ctx context.Context, tenantID, id int64) (*GenerationTopicOverride, error)
	GetGenerationTopicOverrideByTopicKey(ctx context.Context, tenantID int64, platform string, topicKey string) (*GenerationTopicOverride, error)
	CreateGenerationTopicOverride(ctx context.Context, item *GenerationTopicOverride) (*GenerationTopicOverride, error)
	UpdateGenerationTopicOverride(ctx context.Context, item *GenerationTopicOverride) (*GenerationTopicOverride, error)
	UpdateGenerationTopicOverrideStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*GenerationTopicOverride, error)
	DeleteGenerationTopicOverride(ctx context.Context, tenantID, id int64) error
}

type listingGenerationTopicOverride struct {
	ID                             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID                       int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID                    string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Platform                       string     `gorm:"column:platform;type:varchar(32);not null;index"`
	TopicKey                       string     `gorm:"column:topic_key;type:varchar(64);not null;index"`
	AdditionalPromptDirectivesJSON string     `gorm:"column:additional_prompt_directives_json;type:text"`
	AdditionalLexiconJSON          string     `gorm:"column:additional_lexicon_json;type:text"`
	Remark                         string     `gorm:"column:remark"`
	Status                         int16      `gorm:"column:status;not null;default:0;index"`
	Creator                        string     `gorm:"column:creator"`
	CreatedBy                      string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime                     *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater                        string     `gorm:"column:updater"`
	UpdatedBy                      string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime                     *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted                        int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingGenerationTopicOverride) TableName() string {
	return "listing_generation_topic_override"
}

func (r listingGenerationTopicOverride) toGenerationTopicOverride() GenerationTopicOverride {
	return GenerationTopicOverride{
		ID:                          r.ID,
		TenantID:                    r.TenantID,
		Platform:                    r.Platform,
		TopicKey:                    r.TopicKey,
		AdditionalPromptDirectives:  decodeStringListJSON(r.AdditionalPromptDirectivesJSON),
		AdditionalLexiconByLanguage: decodeStringMapListJSON(r.AdditionalLexiconJSON),
		Remark:                      r.Remark,
		Status:                      r.Status,
		CreateTime:                  r.CreateTime,
		UpdateTime:                  r.UpdateTime,
	}
}

func listingGenerationTopicOverrideFromGenerationTopicOverride(item *GenerationTopicOverride) listingGenerationTopicOverride {
	if item == nil {
		return listingGenerationTopicOverride{}
	}
	return listingGenerationTopicOverride{
		ID:                             item.ID,
		TenantID:                       item.TenantID,
		Platform:                       strings.TrimSpace(item.Platform),
		TopicKey:                       strings.TrimSpace(item.TopicKey),
		AdditionalPromptDirectivesJSON: encodeStringListJSON(item.AdditionalPromptDirectives),
		AdditionalLexiconJSON:          encodeStringMapListJSON(item.AdditionalLexiconByLanguage),
		Remark:                         strings.TrimSpace(item.Remark),
		Status:                         item.Status,
	}
}

func applyGenerationTopicOverrideAuditFields(row *listingGenerationTopicOverride, userID string, includeCreate bool) {
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

func applyGenerationTopicOverrideQuery(db *gorm.DB, query GenerationTopicOverrideQuery) *gorm.DB {
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

func encodeStringListJSON(values []string) string {
	normalized := normalizeStringList(values)
	if len(normalized) == 0 {
		return ""
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func decodeStringListJSON(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	return normalizeStringList(items)
}

func encodeStringMapListJSON(values map[string][]string) string {
	normalized := normalizeLexiconMap(values)
	if len(normalized) == 0 {
		return ""
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func decodeStringMapListJSON(value string) map[string][]string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	var items map[string][]string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	return normalizeLexiconMap(items)
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeLexiconMap(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string][]string, len(values))
	keys := make([]string, 0, len(values))
	for language, words := range values {
		normalizedLanguage := strings.TrimSpace(strings.ToLower(language))
		if normalizedLanguage == "" {
			continue
		}
		normalizedWords := normalizeStringList(words)
		if len(normalizedWords) == 0 {
			continue
		}
		normalized[normalizedLanguage] = normalizedWords
		keys = append(keys, normalizedLanguage)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(keys)
	ordered := make(map[string][]string, len(keys))
	for _, key := range keys {
		ordered[key] = normalized[key]
	}
	return ordered
}
