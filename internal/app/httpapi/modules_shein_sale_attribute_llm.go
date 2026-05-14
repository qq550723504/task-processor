package httpapi

import (
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const sheinSaleAttributeClientName = "scorer"

func buildSheinSaleAttributeLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) openaiclient.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, sheinSaleAttributeClientName)
}
