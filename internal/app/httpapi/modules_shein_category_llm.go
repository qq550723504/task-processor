package httpapi

import (
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

func buildSheinCategoryLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, "default")
}
