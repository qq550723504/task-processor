package listingkit

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {
	if s.aiCredentialStore == nil {
		return nil, fmt.Errorf("ai credential store is not configured")
	}
	identity := openaiclient.IdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	requestedScope := normalizeAISettingsScope(scope, identity.UserID)
	userID := aiSettingsUserID(identity, scope)
	credential, resolvedScope, err := s.resolveAISettingsCredential(ctx, tenantID, strings.TrimSpace(identity.UserID), userID, clientName)
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

func (s *service) resolveAISettingsCredential(
	ctx context.Context,
	tenantID string,
	_ string,
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

func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {
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

func aiSettingsUserID(identity openaiclient.Identity, scope string) string {
	if strings.EqualFold(strings.TrimSpace(scope), "tenant") {
		return ""
	}
	return strings.TrimSpace(identity.UserID)
}

func normalizeAISettingsScope(scope string, userID string) string {
	if strings.EqualFold(strings.TrimSpace(scope), "tenant") || userID == "" {
		return "tenant"
	}
	return "user"
}

func normalizeAIClientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return name
}
