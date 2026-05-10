package openai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type AIClientCredential struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	TenantID      string    `json:"tenant_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_ai_client_credential_scope,priority:1"`
	UserID        string    `json:"user_id,omitempty" gorm:"type:varchar(128);not null;default:'';uniqueIndex:idx_ai_client_credential_scope,priority:2"`
	ClientName    string    `json:"client_name" gorm:"type:varchar(64);not null;uniqueIndex:idx_ai_client_credential_scope,priority:3"`
	APIKey        string    `json:"-" gorm:"type:text;not null"`
	BaseURL       string    `json:"base_url" gorm:"type:text;not null"`
	Model         string    `json:"model" gorm:"type:varchar(128);not null"`
	TimeoutSecond int       `json:"timeout_second" gorm:"not null;default:0"`
	Enabled       bool      `json:"enabled" gorm:"not null;default:true"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (AIClientCredential) TableName() string {
	return "ai_client_credentials"
}

type GormCredentialResolver struct {
	db *gorm.DB
}

func NewGormCredentialResolver(db *gorm.DB) *GormCredentialResolver {
	return &GormCredentialResolver{db: db}
}

func (r *GormCredentialResolver) SaveCredential(ctx context.Context, credential AIClientCredential) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("ai credential resolver database is nil")
	}
	credential.TenantID = strings.TrimSpace(credential.TenantID)
	credential.UserID = strings.TrimSpace(credential.UserID)
	credential.ClientName = normalizeClientName(credential.ClientName)
	credential.APIKey = strings.TrimSpace(credential.APIKey)
	credential.BaseURL = strings.TrimSpace(credential.BaseURL)
	credential.Model = strings.TrimSpace(credential.Model)
	if credential.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if credential.ClientName == "" {
		return fmt.Errorf("client_name is required")
	}
	if credential.APIKey == "" {
		return fmt.Errorf("api_key is required")
	}
	if credential.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	if credential.Model == "" {
		return fmt.Errorf("model is required")
	}
	updates := map[string]any{
		"api_key":        credential.APIKey,
		"base_url":       credential.BaseURL,
		"model":          credential.Model,
		"timeout_second": credential.TimeoutSecond,
		"enabled":        credential.Enabled,
	}
	return r.db.WithContext(ctx).Where(
		"tenant_id = ? AND user_id = ? AND client_name = ?",
		credential.TenantID,
		credential.UserID,
		credential.ClientName,
	).Assign(updates).FirstOrCreate(&credential).Error
}

func (r *GormCredentialResolver) GetCredential(ctx context.Context, tenantID, userID, clientName string) (*AIClientCredential, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("ai credential resolver database is nil")
	}
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	clientName = normalizeClientName(clientName)
	if tenantID == "" {
		return nil, nil
	}
	var credential AIClientCredential
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND client_name = ?", tenantID, userID, clientName).
		First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

func (r *GormCredentialResolver) ResolveClientConfig(ctx context.Context, clientName string, fallback *ClientConfig) (*ResolvedClientConfig, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("ai credential resolver database is nil")
	}
	identity := IdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	userID := strings.TrimSpace(identity.UserID)
	clientName = normalizeClientName(clientName)
	if tenantID == "" || clientName == "" {
		return nil, nil
	}
	if userID != "" {
		credential, err := r.findCredential(ctx, tenantID, userID, clientName)
		if err != nil {
			return nil, err
		}
		if credential != nil {
			return credential.toResolvedClientConfig(fallback), nil
		}
	}
	credential, err := r.findCredential(ctx, tenantID, "", clientName)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		return nil, nil
	}
	return credential.toResolvedClientConfig(fallback), nil
}

func (r *GormCredentialResolver) findCredential(ctx context.Context, tenantID, userID, clientName string) (*AIClientCredential, error) {
	var credential AIClientCredential
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND client_name = ? AND enabled = ?", tenantID, userID, clientName, true).
		First(&credential).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

func (c AIClientCredential) toResolvedClientConfig(fallback *ClientConfig) *ResolvedClientConfig {
	cfg := cloneClientConfig(fallback)
	if cfg == nil {
		cfg = &ClientConfig{}
	}
	if c.APIKey != "" {
		cfg.APIKey = c.APIKey
	}
	if c.BaseURL != "" {
		cfg.BaseURL = c.BaseURL
	}
	if c.Model != "" {
		cfg.Model = c.Model
	}
	if c.TimeoutSecond > 0 {
		cfg.Timeout = time.Duration(c.TimeoutSecond) * time.Second
	}
	if cfg.MaxRetries == 0 && fallback != nil {
		cfg.MaxRetries = fallback.MaxRetries
	}
	if cfg.RetryDelay == 0 && fallback != nil {
		cfg.RetryDelay = fallback.RetryDelay
	}
	return &ResolvedClientConfig{
		CacheKey: fmt.Sprintf("db:%d:%d:%s", c.ID, c.UpdatedAt.UnixNano(), c.ClientName),
		Config:   cfg,
	}
}

func cloneClientConfig(cfg *ClientConfig) *ClientConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	return &cloned
}

func normalizeClientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return name
}
