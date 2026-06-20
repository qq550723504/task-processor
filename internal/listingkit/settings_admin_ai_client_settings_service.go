package listingkit

import (
	"context"
	"fmt"
	"strings"
)

func (s *settingsAdminService) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {
	if s.aiCredentialStore == nil {
		return nil, fmt.Errorf("ai credential store is not configured")
	}
	identity := RequestIdentityFromContext(ctx)
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
	settings.APIStyle = credential.APIStyle
	settings.TimeoutSecond = credential.TimeoutSecond
	settings.Enabled = credential.Enabled
	settings.UpdatedAt = credential.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	return settings, nil
}

func (s *settingsAdminService) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {
	if s.aiCredentialStore == nil {
		return nil, fmt.Errorf("ai credential store is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("ai settings request cannot be nil")
	}
	identity := RequestIdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	userID := aiSettingsUserID(identity, req.Scope)
	existing, err := s.aiCredentialStore.GetCredential(ctx, tenantID, userID, req.ClientName)
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		if existing != nil {
			apiKey = existing.APIKey
		}
	}
	if req.TimeoutSecond <= 0 && existing != nil {
		req.TimeoutSecond = existing.TimeoutSecond
	}
	credential := AIClientCredential{
		TenantID:      tenantID,
		UserID:        userID,
		ClientName:    normalizeAIClientName(req.ClientName),
		APIKey:        apiKey,
		BaseURL:       req.BaseURL,
		Model:         req.Model,
		APIStyle:      strings.TrimSpace(req.APIStyle),
		TimeoutSecond: req.TimeoutSecond,
		Enabled:       req.Enabled,
	}
	if err := s.aiCredentialStore.SaveCredential(ctx, credential); err != nil {
		return nil, err
	}
	return s.GetAIClientSettings(ctx, req.Scope, req.ClientName)
}

func (s *settingsAdminService) resolveAISettingsCredential(
	ctx context.Context,
	tenantID string,
	requestedUserID string,
	clientName string,
) (*AIClientCredential, string, error) {
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
