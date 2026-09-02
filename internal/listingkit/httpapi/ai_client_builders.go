package httpapi

import (
	"task-processor/internal/ai"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/integration/openai"
)

func buildStrictListingKitChatClient(cfg *config.Config, resolver openaiclient.ClientConfigResolver, clientName string) ai.ChatCompleter {
	return &strictListingKitChatClient{
		clientName: clientName,
		resolver:   resolver,
		fallback:   buildListingKitClientFallback(cfg, clientName),
		cache:      make(map[string]*openaiclient.Client),
	}
}
