package httpapi

import (
	"strings"

	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func buildListingKitClientFallback(cfg *config.Config, clientName string) *openaiclient.ClientConfig {
	if cfg == nil {
		return nil
	}
	base := cfg.OpenAI.ToClientConfig()
	if named, ok := cfg.OpenAI.ToClientConfigs()[normalizeListingKitClientName(clientName)]; ok && named != nil {
		base = named
	}
	return sanitizeListingKitClientFallback(base)
}

func sanitizeListingKitClientFallback(cfg *openaiclient.ClientConfig) *openaiclient.ClientConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	cloned.APIKey = ""
	cloned.BaseURL = ""
	cloned.Model = ""
	return &cloned
}

func normalizeListingKitClientName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "default"
	}
	return name
}
