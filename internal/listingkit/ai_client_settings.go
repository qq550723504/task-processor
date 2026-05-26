package listingkit

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func (s *service) GetAIClientSettings(ctx context.Context, scope string, clientName string) (*AIClientSettings, error) {
	return s.settingsAdminOrDefault().GetAIClientSettings(ctx, scope, clientName)
}

func (s *service) UpdateAIClientSettings(ctx context.Context, req *AIClientSettings) (*AIClientSettings, error) {
	return s.settingsAdminOrDefault().UpdateAIClientSettings(ctx, req)
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
