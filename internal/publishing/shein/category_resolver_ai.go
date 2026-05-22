package shein

import openaiclient "task-processor/internal/infra/clients/openai"

func NewCategoryResolverWithAI(api CategoryAPI, llmClient ...openaiclient.ChatCompleter) CategoryResolver {
	var suggestFallback categorySuggestFallback
	var treeFallback categoryTreeFallback
	var semanticVerifier categorySemanticVerifier
	if len(llmClient) > 0 {
		suggestFallback = newAICategorySuggestFallback(llmClient[0])
		treeFallback = newAICategoryTreeFallback(llmClient[0])
		semanticVerifier = newAICategorySemanticVerifier(llmClient[0])
	}
	return NewCategoryResolverWithSemanticVerifier(api, suggestFallback, treeFallback, semanticVerifier)
}
