package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type settingsAdminServiceConfig struct {
	storeProfileRepo     StoreProfileRepository
	routingSettingsRepo  StoreRoutingSettingsRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
}

type settingsAdminService struct {
	storeProfileRepo     StoreProfileRepository
	routingSettingsRepo  StoreRoutingSettingsRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
}

func newSettingsAdminService(config settingsAdminServiceConfig) *settingsAdminService {
	return &settingsAdminService{
		storeProfileRepo:     config.storeProfileRepo,
		routingSettingsRepo:  config.routingSettingsRepo,
		aiCredentialStore:    config.aiCredentialStore,
		currentSheinSettings: config.currentSheinSettings,
		mutateSheinSettings:  config.mutateSheinSettings,
		listStoreOptions:     config.listStoreOptions,
	}
}

func (s *settingsAdminService) ListSheinStoreProfiles(ctx context.Context) ([]ListingKitStoreProfile, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	items, err := s.storeProfileRepo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return s.attachStoreOptions(ctx, items), nil
}

func (s *settingsAdminService) UpsertSheinStoreProfile(ctx context.Context, req *ListingKitStoreProfile) (*ListingKitStoreProfile, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("store profile is required")
	}
	if req.StoreID <= 0 {
		return nil, fmt.Errorf("store_id is required")
	}
	profile := *req
	profile.TenantID = tenantID
	normalizeStoreProfile(&profile)
	saved, err := s.storeProfileRepo.Upsert(ctx, &profile)
	if err != nil {
		return nil, err
	}
	items := s.attachStoreOptions(ctx, []ListingKitStoreProfile{*saved})
	if len(items) == 0 {
		return saved, nil
	}
	return cloneStoreProfile(&items[0]), nil
}

func (s *settingsAdminService) DeleteSheinStoreProfile(ctx context.Context, id int64) error {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return fmt.Errorf("tenant id is required")
	}
	if id <= 0 {
		return fmt.Errorf("profile id is required")
	}
	return s.storeProfileRepo.Delete(ctx, tenantID, id)
}

func (s *settingsAdminService) GetSheinStoreRoutingSettings(ctx context.Context) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	return s.routingSettingsRepo.GetByTenant(ctx, tenantID)
}

func (s *settingsAdminService) UpdateSheinStoreRoutingSettings(ctx context.Context, req *ListingKitStoreRoutingSettings) (*ListingKitStoreRoutingSettings, error) {
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant id is required")
	}
	if req == nil {
		return nil, fmt.Errorf("legacy store routing settings are required")
	}
	settings := *req
	settings.TenantID = tenantID
	settings = normalizeStoreRoutingSettings(settings)
	return s.routingSettingsRepo.Upsert(ctx, &settings)
}

func (s *settingsAdminService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {
	if s.aiCredentialStore == nil {
		return nil, fmt.Errorf("ai credential store is not configured")
	}
	identity := openaiclient.IdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	requestedScope := normalizeAISettingsScope(scope, identity.UserID)
	userID := aiSettingsUserID(identity, scope)
	credential, resolvedScope, err := s.resolveAISettingsCredential(ctx, tenantID, userID, clientName)
	if err != nil {
		return nil, err
	}
	settings := &AIClientSettings{
		Scope:         requestedScope,
		ClientName:    normalizeAIClientName(clientName),
		Enabled:       true,
		ResolvedScope: resolvedScope,
	}
	if credential == nil {
		return settings, nil
	}
	settings.APIKeySet = credential.APIKey != ""
	settings.BaseURL = credential.BaseURL
	settings.Model = credential.Model
	settings.Enabled = credential.Enabled
	settings.UpdatedAt = credential.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	return settings, nil
}

func (s *settingsAdminService) GetSheinSettings(ctx context.Context) (*SheinSettings, error) {
	if s == nil || s.currentSheinSettings == nil {
		return nil, fmt.Errorf("shein settings are not configured")
	}
	settings := s.currentSheinSettings()
	settings.AvailableStores = s.listStoreOptions(ctx)
	return &settings, nil
}

func (s *settingsAdminService) UpdateSheinSettings(ctx context.Context, req *SheinSettings) (*SheinSettings, error) {
	if req == nil {
		return s.GetSheinSettings(ctx)
	}
	if s == nil || s.mutateSheinSettings == nil {
		return nil, fmt.Errorf("shein settings are not configured")
	}
	settings := s.mutateSheinSettings(func(settings *SheinSettings) {
		if req.DefaultStoreID > 0 {
			settings.DefaultStoreID = req.DefaultStoreID
		}
		if value := strings.ToUpper(strings.TrimSpace(req.Site)); value != "" {
			settings.Site = value
		}
		if value := strings.TrimSpace(req.WarehouseCode); value != "" {
			settings.WarehouseCode = value
		}
		if req.DefaultStock > 0 {
			settings.DefaultStock = req.DefaultStock
		}
		if value := strings.ToLower(strings.TrimSpace(req.DefaultSubmitMode)); value == "publish" || value == "save_draft" {
			settings.DefaultSubmitMode = value
		}
		settings.Pricing = normalizeSheinPricingRule(req.Pricing, settings.Pricing)
		now := time.Now()
		settings.UpdatedAt = &now
	})
	settings.AvailableStores = s.listStoreOptions(ctx)
	return &settings, nil
}

func (s *settingsAdminService) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {
	if s.aiCredentialStore == nil {
		return nil, fmt.Errorf("ai credential store is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("ai settings request cannot be nil")
	}
	identity := openaiclient.IdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	userID := aiSettingsUserID(identity, req.Scope)
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		existing, err := s.aiCredentialStore.GetCredential(ctx, tenantID, userID, req.ClientName)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			apiKey = existing.APIKey
		}
	}
	credential := openaiclient.AIClientCredential{
		TenantID:      tenantID,
		UserID:        userID,
		ClientName:    normalizeAIClientName(req.ClientName),
		APIKey:        apiKey,
		BaseURL:       req.BaseURL,
		Model:         req.Model,
		TimeoutSecond: 0,
		Enabled:       req.Enabled,
	}
	if err := s.aiCredentialStore.SaveCredential(ctx, credential); err != nil {
		return nil, err
	}
	return s.GetAIClientSettings(ctx, req.Scope, req.ClientName)
}

func (s *settingsAdminService) attachStoreOptions(ctx context.Context, items []ListingKitStoreProfile) []ListingKitStoreProfile {
	if len(items) == 0 || s.listStoreOptions == nil {
		return items
	}
	options := s.listStoreOptions(ctx)
	if len(options) == 0 {
		return items
	}
	byID := make(map[int64]SheinStoreOption, len(options))
	for _, option := range options {
		byID[option.ID] = option
	}
	for idx := range items {
		option, ok := byID[items[idx].StoreID]
		if !ok {
			continue
		}
		copyOption := option
		items[idx].Store = &copyOption
	}
	return items
}

func (s *settingsAdminService) resolveAISettingsCredential(
	ctx context.Context,
	tenantID string,
	requestedUserID string,
	clientName string,
) (*openaiclient.AIClientCredential, string, error) {
	if tenantID == "" {
		return nil, "", nil
	}
	if requestedUserID != "" {
		credential, err := s.aiCredentialStore.GetCredential(ctx, tenantID, requestedUserID, clientName)
		if err != nil {
			return nil, "", err
		}
		if credential != nil {
			return credential, "user", nil
		}
	}
	credential, err := s.aiCredentialStore.GetCredential(ctx, tenantID, "", clientName)
	if err != nil {
		return nil, "", err
	}
	if credential != nil {
		return credential, "tenant", nil
	}
	return nil, "", nil
}
