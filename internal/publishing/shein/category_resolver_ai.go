package shein

func NewCategoryResolverWithAI(api CategoryAPI, aiConfig CategoryAIConfig) CategoryResolver {
	var suggestFallback categorySuggestFallback
	var treeFallback categoryTreeFallback
	var semanticVerifier categorySemanticVerifier
	if aiConfig.Selector != nil {
		suggestFallback = newAICategorySuggestFallback(aiConfig.Selector)
		treeFallback = newAICategoryTreeFallback(aiConfig.Selector)
	}
	if aiConfig.SemanticVerifier != nil {
		semanticVerifier = newAICategorySemanticVerifier(aiConfig.SemanticVerifier)
	}
	return NewCategoryResolverWithSemanticVerifier(api, suggestFallback, treeFallback, semanticVerifier)
}
