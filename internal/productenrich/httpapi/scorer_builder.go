package httpapi

import (
	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
)

const productScorerClientName = "scorer"

func buildLLMScorerConfig(cfg *config.Config, llmMgr productenrich.LLMManager, scoreCache productenrich.LLMScoreCache) *productenrich.LLMScorerConfig {
	scorerCfg := &productenrich.LLMScorerConfig{
		LLMManager: llmMgr,
		ScoreCache: scoreCache,
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

func buildLLMScorerWithCache(cfg *config.Config, llmMgr productenrich.LLMManager, scoreCache productenrich.LLMScoreCache) productenrich.LLMScorer {
	return productenrich.NewLLMScorer(buildLLMScorerConfig(cfg, llmMgr, scoreCache))
}
