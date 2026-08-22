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
	return buildLLMScorerWithCapabilities(cfg, llmMgr, scoreCache, nil, nil)
}

func buildLLMScorerWithCapabilities(cfg *config.Config, llmMgr productenrich.LLMManager, scoreCache productenrich.LLMScoreCache, textGenerator productenrich.ScoringTextGenerator, imageAnalyzer productenrich.ScoringImageAnalyzer) productenrich.LLMScorer {
	config := buildLLMScorerConfig(cfg, llmMgr, scoreCache)
	config.TextGenerator = textGenerator
	config.ImageAnalyzer = imageAnalyzer
	return productenrich.NewLLMScorer(config)
}
