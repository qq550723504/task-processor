package httpapi

import (
	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
)

const productScorerClientName = "scorer"

func buildProductLLMScorerConfig(cfg *config.Config, llmMgr productenrich.LLMManager) *productenrich.LLMScorerConfig {
	scorerCfg := &productenrich.LLMScorerConfig{
		LLMManager: llmMgr,
	}

	if cfg == nil {
		return scorerCfg
	}
	if _, ok := cfg.OpenAI.Clients[productScorerClientName]; ok {
		scorerCfg.TextClient = productScorerClientName
		scorerCfg.VisionClient = productScorerClientName
	}

	return scorerCfg
}

func buildProductLLMScorer(cfg *config.Config, llmMgr productenrich.LLMManager) productenrich.LLMScorer {
	return productenrich.NewLLMScorer(buildProductLLMScorerConfig(cfg, llmMgr))
}
