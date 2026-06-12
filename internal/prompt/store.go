package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sharedtenantctx "task-processor/internal/shared/tenantctx"

	"gorm.io/gorm"
)

var ErrTenantPromptNotFound = errors.New("tenant prompt not found")

type TenantPromptTemplate struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TenantID  string    `json:"tenant_id" gorm:"type:varchar(128);not null;uniqueIndex:idx_tenant_prompt_template_scope,priority:1"`
	Key       string    `json:"key" gorm:"type:varchar(255);not null;uniqueIndex:idx_tenant_prompt_template_scope,priority:2"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	Version   string    `json:"version,omitempty" gorm:"type:varchar(64);not null;default:''"`
	Enabled   bool      `json:"enabled" gorm:"not null;index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (TenantPromptTemplate) TableName() string {
	return "tenant_prompt_templates"
}

type TenantPromptStore interface {
	GetEnabled(ctx context.Context, tenantID string, key string) (*TenantPromptTemplate, error)
	ListTenant(ctx context.Context, tenantID string) ([]TenantPromptTemplate, error)
	SetEnabled(ctx context.Context, tenantID string, key string, enabled bool) error
	Upsert(ctx context.Context, tmpl TenantPromptTemplate) error
}

type GormTenantPromptStore struct {
	db *gorm.DB
}

func NewGormTenantPromptStore(db *gorm.DB) *GormTenantPromptStore {
	return &GormTenantPromptStore{db: db}
}

func (s *GormTenantPromptStore) GetEnabled(ctx context.Context, tenantID string, key string) (*TenantPromptTemplate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("tenant prompt store database is nil")
	}
	tenantID = sharedtenantctx.NormalizeTenantID(tenantID)
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrTenantPromptNotFound
	}
	var tmpl TenantPromptTemplate
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND key = ? AND enabled = ?", tenantID, key, true).
		First(&tmpl).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTenantPromptNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tmpl, nil
}

func (s *GormTenantPromptStore) ListTenant(ctx context.Context, tenantID string) ([]TenantPromptTemplate, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("tenant prompt store database is nil")
	}
	tenantID = sharedtenantctx.NormalizeTenantID(tenantID)
	var templates []TenantPromptTemplate
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("key asc").
		Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func (s *GormTenantPromptStore) SetEnabled(ctx context.Context, tenantID string, key string, enabled bool) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("tenant prompt store database is nil")
	}
	tenantID = sharedtenantctx.NormalizeTenantID(tenantID)
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("prompt key is required")
	}
	result := s.db.WithContext(ctx).
		Model(&TenantPromptTemplate{}).
		Where("tenant_id = ? AND key = ?", tenantID, key).
		Update("enabled", enabled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTenantPromptNotFound
	}
	return nil
}

func (s *GormTenantPromptStore) Upsert(ctx context.Context, tmpl TenantPromptTemplate) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("tenant prompt store database is nil")
	}
	tmpl.TenantID = sharedtenantctx.NormalizeTenantID(tmpl.TenantID)
	tmpl.Key = strings.TrimSpace(tmpl.Key)
	tmpl.Content = strings.TrimSpace(tmpl.Content)
	tmpl.Version = strings.TrimSpace(tmpl.Version)
	if tmpl.Key == "" {
		return fmt.Errorf("prompt key is required")
	}
	if tmpl.Content == "" {
		return fmt.Errorf("prompt content is required")
	}
	updates := map[string]any{
		"content": tmpl.Content,
		"version": tmpl.Version,
		"enabled": tmpl.Enabled,
	}
	return s.db.WithContext(ctx).Where(
		"tenant_id = ? AND key = ?",
		tmpl.TenantID,
		tmpl.Key,
	).Assign(updates).FirstOrCreate(&tmpl).Error
}
