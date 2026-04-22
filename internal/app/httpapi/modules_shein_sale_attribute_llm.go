package httpapi

import (
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
)

const sheinSaleAttributeClientName = "scorer"

func buildSheinSaleAttributeLLMClient(cfg *config.Config, mgr *openaiclient.Manager) openaiclient.ChatCompleter {
	if mgr == nil {
		return nil
	}
	if cfg != nil {
		if _, ok := cfg.OpenAI.Clients[sheinSaleAttributeClientName]; ok {
			if client, err := mgr.GetClient(sheinSaleAttributeClientName); err == nil {
				return client
			}
		}
	}
	return mgr.GetDefaultClient()
}
