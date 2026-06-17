package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"task-processor/internal/listingkit"
)

var (
	errUnsupportedSettingsNamespace = errors.New("unsupported settings namespace")
)

type settingsNamespaceQuery struct {
	TenantID   string
	Scope      string
	ClientName string
}

type settingsScopeDefinition struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type settingsFieldDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

type settingsNamespaceSchema struct {
	Namespace            string                    `json:"namespace"`
	Label                string                    `json:"label"`
	Description          string                    `json:"description"`
	SupportedScopes      []settingsScopeDefinition `json:"supported_scopes,omitempty"`
	Fields               []settingsFieldDefinition `json:"fields,omitempty"`
	SupportsStatusToggle bool                      `json:"supports_status_toggle,omitempty"`
}

type settingsService struct {
	service settingsHandlerService
}

type settingsNamespaceService interface {
	Get(ctx context.Context, namespace string, query settingsNamespaceQuery) (any, error)
	Health(ctx context.Context) (listingkit.SettingsHealthPage, error)
	ListSchemas() []settingsNamespaceSchema
	GetSchema(namespace string) (*settingsNamespaceSchema, error)
	Update(ctx context.Context, namespace string, query settingsNamespaceQuery, payload []byte) (any, error)
}

type settingsHandlerService interface {
	GetSheinSettings(ctx context.Context) (*listingkit.SheinSettings, error)
	UpdateSheinSettings(ctx context.Context, req *listingkit.SheinSettings) (*listingkit.SheinSettings, error)
	GetAIClientSettings(ctx context.Context, scope string, clientName string) (*listingkit.AIClientSettings, error)
	UpdateAIClientSettings(ctx context.Context, req *listingkit.AIClientSettings) (*listingkit.AIClientSettings, error)
}

type settingsHealthProbeProvider interface {
	GetSettingsHealthProbes(ctx context.Context) listingkit.SettingsHealthProbes
}

var settingsNamespaceSchemas = []settingsNamespaceSchema{
	{
		Namespace:   "shein",
		Label:       "SHEIN 配置",
		Description: "当前租户的 SHEIN 默认店铺、站点、库存和提交规则。",
		SupportedScopes: []settingsScopeDefinition{
			{ID: "tenant", Label: "租户", Description: "当前租户统一使用"},
		},
		Fields: []settingsFieldDefinition{
			{Key: "default_store_id", Label: "默认店铺", Type: "number"},
			{Key: "site", Label: "站点", Type: "string"},
			{Key: "warehouse_code", Label: "仓库编码", Type: "string"},
			{Key: "default_stock", Label: "默认库存", Type: "number"},
			{Key: "default_submit_mode", Label: "默认提交方式", Type: "string"},
			{Key: "pricing", Label: "价格规则", Type: "object"},
		},
	},
	{
		Namespace:   "ai",
		Label:       "AI 配置",
		Description: "租户级和用户级模型 endpoint、key、model 与超时配置。",
		SupportedScopes: []settingsScopeDefinition{
			{ID: "tenant", Label: "租户", Description: "作为当前租户默认配置"},
			{ID: "user", Label: "用户", Description: "覆盖租户默认配置"},
		},
		Fields: []settingsFieldDefinition{
			{Key: "client_name", Label: "客户端名称", Type: "string", Required: true},
			{Key: "base_url", Label: "Endpoint", Type: "string", Required: true},
			{Key: "model", Label: "模型", Type: "string", Required: true},
			{Key: "api_key", Label: "API Key", Type: "secret"},
			{Key: "enabled", Label: "启用状态", Type: "boolean"},
		},
	},
}

func newSettingsService(service settingsHandlerService) *settingsService {
	return &settingsService{
		service: service,
	}
}

func (s *settingsService) Get(ctx context.Context, namespace string, query settingsNamespaceQuery) (any, error) {
	switch namespace {
	case "shein":
		return s.service.GetSheinSettings(ctx)
	case "ai":
		return s.service.GetAIClientSettings(ctx, query.Scope, query.ClientName)
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedSettingsNamespace, namespace)
	}
}

func (s *settingsService) Health(ctx context.Context) (listingkit.SettingsHealthPage, error) {
	defaultAI, err := s.service.GetAIClientSettings(ctx, "tenant", "default")
	if err != nil {
		return listingkit.SettingsHealthPage{}, err
	}
	imageAI, err := s.service.GetAIClientSettings(ctx, "tenant", "image")
	if err != nil {
		return listingkit.SettingsHealthPage{}, err
	}
	shein, err := s.service.GetSheinSettings(ctx)
	if err != nil {
		return listingkit.SettingsHealthPage{}, err
	}
	return listingkit.BuildSettingsHealth(listingkit.SettingsHealthInputs{
		DefaultAI: defaultAI,
		ImageAI:   imageAI,
		Shein:     shein,
		Probes:    s.healthProbes(ctx),
	}), nil
}

func (s *settingsService) healthProbes(ctx context.Context) listingkit.SettingsHealthProbes {
	provider, ok := s.service.(settingsHealthProbeProvider)
	if !ok || provider == nil {
		return listingkit.SettingsHealthProbes{}
	}
	return provider.GetSettingsHealthProbes(ctx)
}

func (s *settingsService) ListSchemas() []settingsNamespaceSchema {
	return append([]settingsNamespaceSchema(nil), settingsNamespaceSchemas...)
}

func (s *settingsService) GetSchema(namespace string) (*settingsNamespaceSchema, error) {
	for _, schema := range settingsNamespaceSchemas {
		if schema.Namespace == namespace {
			copySchema := schema
			return &copySchema, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", errUnsupportedSettingsNamespace, namespace)
}

func (s *settingsService) Update(ctx context.Context, namespace string, query settingsNamespaceQuery, payload []byte) (any, error) {
	switch namespace {
	case "shein":
		var req listingkit.SheinSettings
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		return s.service.UpdateSheinSettings(ctx, &req)
	case "ai":
		var req listingkit.AIClientSettings
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}
		return s.service.UpdateAIClientSettings(ctx, &req)
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedSettingsNamespace, namespace)
	}
}
