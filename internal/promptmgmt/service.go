package promptmgmt

import (
	"context"
	"errors"
	"strings"

	"task-processor/internal/listingkit/tenantctx"
	"task-processor/internal/prompt"
)

var (
	ErrServiceUnavailable = errors.New("tenant prompt store is not configured")
	ErrTemplateNotFound   = errors.New("prompt template not found")
)

type Service interface {
	ListTemplateCatalog() []TemplateSchema
	GetTemplateSchema(key string) (*TemplateSchema, error)
	ListTenantTemplates(ctx context.Context, tenantID string) ([]Template, error)
	UpsertTenantTemplate(ctx context.Context, tmpl UpsertTemplateInput) (*Template, error)
	SetTenantTemplateStatus(ctx context.Context, tenantID string, key string, enabled bool) (*TemplateStatus, error)
}

type service struct {
	store prompt.TenantPromptStore
}

func NewService(store prompt.TenantPromptStore) Service {
	return &service{store: store}
}

func (s *service) ListTemplateCatalog() []TemplateSchema {
	return buildTemplateCatalog()
}

func (s *service) GetTemplateSchema(key string) (*TemplateSchema, error) {
	key = strings.TrimSpace(key)
	for _, schema := range buildTemplateCatalog() {
		if schema.Key == key {
			copySchema := schema
			return &copySchema, nil
		}
	}
	return nil, ErrTemplateNotFound
}

func (s *service) ListTenantTemplates(ctx context.Context, tenantID string) ([]Template, error) {
	if s.store == nil {
		return nil, ErrServiceUnavailable
	}
	items, err := s.store.ListTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	templates := make([]Template, 0, len(items))
	for _, item := range items {
		templates = append(templates, fromStoreTemplate(item))
	}
	return templates, nil
}

func (s *service) UpsertTenantTemplate(ctx context.Context, input UpsertTemplateInput) (*Template, error) {
	if s.store == nil {
		return nil, ErrServiceUnavailable
	}
	tmpl := toStoreTemplate(input)
	if _, err := s.GetTemplateSchema(tmpl.Key); err != nil {
		return nil, err
	}
	if err := s.store.Upsert(ctx, tmpl); err != nil {
		return nil, err
	}
	if tmpl.Enabled {
		saved, err := s.store.GetEnabled(ctx, tmpl.TenantID, tmpl.Key)
		if err == nil {
			result := fromStoreTemplate(*saved)
			return &result, nil
		}
	}
	result := fromStoreTemplate(tmpl)
	return &result, nil
}

func (s *service) SetTenantTemplateStatus(ctx context.Context, tenantID string, key string, enabled bool) (*TemplateStatus, error) {
	if s.store == nil {
		return nil, ErrServiceUnavailable
	}
	tenantID = tenantctx.NormalizeTenantID(tenantID)
	key = strings.TrimSpace(key)
	if err := s.store.SetEnabled(ctx, tenantID, key, enabled); err != nil {
		if errors.Is(err, prompt.ErrTenantPromptNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return &TemplateStatus{
		TenantID: tenantID,
		Key:      key,
		Enabled:  enabled,
	}, nil
}

func toStoreTemplate(input UpsertTemplateInput) prompt.TenantPromptTemplate {
	return prompt.TenantPromptTemplate{
		TenantID: tenantctx.NormalizeTenantID(input.TenantID),
		Key:      strings.TrimSpace(input.Key),
		Content:  strings.TrimSpace(input.Content),
		Version:  strings.TrimSpace(input.Version),
		Enabled:  input.Enabled,
	}
}

func fromStoreTemplate(tmpl prompt.TenantPromptTemplate) Template {
	return Template{
		ID:        tmpl.ID,
		TenantID:  tmpl.TenantID,
		Key:       tmpl.Key,
		Content:   tmpl.Content,
		Version:   tmpl.Version,
		Enabled:   tmpl.Enabled,
		CreatedAt: tmpl.CreatedAt,
		UpdatedAt: tmpl.UpdatedAt,
	}
}
