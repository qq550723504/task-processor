package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
)

// buildService 组装 productenrich.ProductService（全内存，无 Worker Pool）。
func buildService(cfg *config.Config) (productenrich.ProductService, error) {
	logger := logrus.StandardLogger()

	llmMgr, err := productenrich.NewLLMManagerAdapter(cfg.OpenAI)
	if err != nil {
		return nil, fmt.Errorf("创建 LLMManager 失败: %w", err)
	}
	logger.Info("✅ OpenAI LLMManager 已初始化")

	productUnderstanding, err := productenrich.NewProductUnderstanding(llmMgr)
	if err != nil {
		return nil, fmt.Errorf("创建 ProductUnderstanding 失败: %w", err)
	}

	jsonGenerator, err := productenrich.NewJSONGenerator(logger, llmMgr)
	if err != nil {
		return nil, fmt.Errorf("创建 JSONGenerator 失败: %w", err)
	}

	variantGenerator, err := productenrich.NewVariantGenerator(llmMgr)
	if err != nil {
		return nil, fmt.Errorf("创建 VariantGenerator 失败: %w", err)
	}

	llmScorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{LLMManager: llmMgr})
	qualityScorer := productenrich.NewQualityScorer(&productenrich.QualityScorerConfig{
		ImageWeight:   0.4,
		TextWeight:    0.3,
		ScrapedWeight: 0.3,
		LLMScorer:     llmScorer,
		EnableLLM:     true,
	})

	inputParser, err := productenrich.NewInputParser(logger, &productenrich.InputParserConfig{},
		productenrich.NewCrawler1688Adapter(cfg))
	if err != nil {
		return nil, fmt.Errorf("创建 InputParser 失败: %w", err)
	}
	logger.Info("✅ InputParser（1688爬虫）已初始化")

	return productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "product_enrich_tasks",
		TaskRepo:             productenrich.NewMemTaskRepository(),
		RedisClient:          productenrich.NewMemRedisClient(),
		InputParser:          inputParser,
		ProductUnderstanding: productUnderstanding,
		JSONGenerator:        jsonGenerator,
		VariantGenerator:     variantGenerator,
		QualityScorer:        qualityScorer,
		StrategySelector:     productenrich.NewStrategySelector(nil),
		ResultValidator:      productenrich.NewResultValidator(),
		EnhancementSuggester: productenrich.NewEnhancementSuggester(),
		InputValidator: productenrich.NewInputValidator(&productenrich.InputValidatorConfig{
			HTTPTimeout: 5 * time.Second,
			MaxWorkers:  10,
		}),
	})
}
