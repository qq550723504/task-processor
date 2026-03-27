package main

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productpipeline "task-processor/internal/productenrich/pipeline"
	"task-processor/internal/productenrich/store"
)

func buildProductModule(logger *logrus.Logger, deps *runtimeDeps) (*productModule, error) {
	jsonGenerator, err := productenrichenrich.NewJSONGenerator(logger, deps.llmMgr)
	if err != nil {
		return nil, fmt.Errorf("创建 JSON 生成器：%w", err)
	}

	variantGenerator, err := productenrichenrich.NewVariantGenerator(deps.llmMgr)
	if err != nil {
		return nil, fmt.Errorf("创建变体生成器：%w", err)
	}

	taskRepo, closers, err := buildProductTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, closers...)

	redisClient, err := buildProductRedisClient(deps.cfg, logger)
	if err != nil {
		return nil, err
	}

	llmScorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{LLMManager: deps.llmMgr})
	qualityScorer := productenrich.NewQualityScorer(&productenrich.QualityScorerConfig{
		ImageWeight:   0.4,
		TextWeight:    0.3,
		ScrapedWeight: 0.3,
		LLMScorer:     llmScorer,
		EnableLLM:     true,
	})
	productCapabilities := productenrich.StrictProductServiceCapabilities()
	productSvc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "product_enrich_tasks",
		TaskRepo:             taskRepo,
		RedisClient:          redisClient,
		Capabilities:         &productCapabilities,
		InputParser:          deps.inputParser,
		ProductUnderstanding: deps.understanding,
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
	if err != nil {
		return nil, fmt.Errorf("创建产品服务：%w", err)
	}

	productProcessor, err := productpipeline.NewProcessor(productSvc, taskRepo, logger, 3)
	if err != nil {
		return nil, fmt.Errorf("创建产品处理器：%w", err)
	}
	productPool := newWorkerPool(productProcessor, deps.cfg)
	productSubmitter := &poolSubmitter{pool: productPool}
	productSvc.SetTaskSubmitter(productSubmitter)
	productProcessor.SetTaskSubmitter(productSubmitter)

	productHandler, err := productapi.NewProductHandler(productSvc)
	if err != nil {
		return nil, fmt.Errorf("创建产品处理器：%w", err)
	}

	return &productModule{handler: productHandler, pool: productPool}, nil
}

func buildProductTaskRepository(cfg *config.Config, logger *logrus.Logger) (productenrich.TaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("创建任务仓库：%w", err)
		}
		return repo, []func() error{closer}, nil
	}
	logger.Warn("未配置数据库，使用内存 productenrich 仓库")
	return store.NewMemTaskRepository(), nil, nil
}

func buildProductRedisClient(cfg *config.Config, logger *logrus.Logger) (productenrich.RedisClient, error) {
	if cfg != nil && cfg.Redis != nil && cfg.Redis.Host != "" {
		redisClient, err := newRedisClient(cfg.Redis, logger)
		if err != nil {
			return nil, fmt.Errorf("创建 Redis 客户端：%w", err)
		}
		return redisClient, nil
	}
	logger.Warn("未配置 Redis，使用内存 productenrich 队列降级方案")
	return productenrich.NewMemRedisClient(), nil
}
