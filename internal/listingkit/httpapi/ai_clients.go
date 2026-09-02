package httpapi

import (
	"fmt"

	"task-processor/internal/ai"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
)

const (
	sheinSaleAttributeClientName = "scorer"
)

func BuildSheinCategoryLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) ai.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, "default")
}

func BuildSheinSaleAttributeLLMClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver) ai.ChatCompleter {
	return buildStrictListingKitChatClient(cfg, resolver, sheinSaleAttributeClientName)
}

func errListingKitAIClientNotConfigured(clientName string) error {
	return fmt.Errorf("listingkit ai client %q is not configured for current tenant/user", normalizeListingKitClientName(clientName))
}
