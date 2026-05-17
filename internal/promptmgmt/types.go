package promptmgmt

import "time"

type Template struct {
	ID        uint      `json:"id,omitempty"`
	TenantID  string    `json:"tenant_id,omitempty"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	Version   string    `json:"version,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type UpsertTemplateInput struct {
	TenantID string `json:"tenant_id,omitempty"`
	Key      string `json:"key"`
	Content  string `json:"content"`
	Version  string `json:"version,omitempty"`
	Enabled  bool   `json:"enabled"`
}

type TemplateStatus struct {
	TenantID string `json:"tenant_id"`
	Key      string `json:"key"`
	Enabled  bool   `json:"enabled"`
}

type TemplateScopeDefinition struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type TemplateVariableDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type TemplateSchema struct {
	Key                    string                       `json:"key"`
	Label                  string                       `json:"label"`
	Description            string                       `json:"description,omitempty"`
	Group                  string                       `json:"group"`
	GroupLabel             string                       `json:"group_label"`
	Category               string                       `json:"category"`
	CategoryLabel          string                       `json:"category_label"`
	SupportedScopes        []TemplateScopeDefinition    `json:"supported_scopes,omitempty"`
	Variables              []TemplateVariableDefinition `json:"variables,omitempty"`
	HasDefaultContent      bool                         `json:"has_default_content"`
	SupportsTenantOverride bool                         `json:"supports_tenant_override"`
}
