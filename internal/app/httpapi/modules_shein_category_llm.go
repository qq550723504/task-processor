package httpapi

import (
	openaiclient "task-processor/internal/infra/clients/openai"
)

func buildSheinCategoryLLMClient(mgr *openaiclient.Manager) openaiclient.ChatCompleter {
	if mgr == nil {
		return nil
	}
	return mgr.GetDefaultClient()
}
