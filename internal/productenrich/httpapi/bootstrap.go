package httpapi

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	"task-processor/internal/infra/redisclient"
	"task-processor/internal/infra/worker"
	platformdatabase "task-processor/internal/platform/database"
	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productpipeline "task-processor/internal/productenrich/pipeline"
	productstore "task-processor/internal/productenrich/store"
)

type Module struct {
	Handler productenrich.ProductHandler
	Pool    worker.WorkerPool
	Service productenrich.ProductService
	Closers []func() error
}

type BuildModuleInput struct {
	Config               *config.Config
	Logger               *logrus.Logger
	LLMManager           productenrich.LLMManager
	TextGenerator        productenrichenrich.TextGenerator
	SpecsGenerator       productenrichenrich.TextGenerator
	VariantsGenerator    productenrichenrich.TextGenerator
	ScoringTextGenerator productenrichenrich.TextGenerator
	ScoringImageAnalyzer productenrichenrich.ImageAnalyzer
	InputParser          productenrich.InputParser
	Understanding        productenrich.ProductUnderstanding
	LLMScorerCache       productenrich.MetricsCollector
}

func BuildModule(input BuildModuleInput) (*Module, error) {
	jsonGenerator, err := productenrichenrich.NewJSONGeneratorWithTextGenerator(input.Logger, input.LLMManager, input.TextGenerator)
	if err != nil {
		return nil, fmt.Errorf("create JSON generator: %w", err)
	}

	variantGenerator, err := productenrichenrich.NewVariantGeneratorWithGenerators(input.LLMManager, input.SpecsGenerator, input.VariantsGenerator)
	if err != nil {
		return nil, fmt.Errorf("create variant generator: %w", err)
	}

	taskRepo, closers, err := buildTaskRepository(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}

	redisClient, err := buildRedisClient(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}
	scoreCache := productenrich.NewLLMScoreCache(redisClient, nil)
	llmScorer := buildLLMScorerWithCapabilities(input.Config, input.LLMManager, scoreCache, input.ScoringTextGenerator, input.ScoringImageAnalyzer)
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
		InputParser:          input.InputParser,
		ProductUnderstanding: input.Understanding,
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
		return nil, fmt.Errorf("create product service: %w", err)
	}

	productProcessor, err := productpipeline.NewProcessor(productSvc, taskRepo, input.Logger, 3)
	if err != nil {
		return nil, fmt.Errorf("create product processor: %w", err)
	}
	productPool := httpbootstrap.NewWorkerPool(productProcessor, input.Config)
	productSubmitter := &httpbootstrap.PoolSubmitter{Pool: productPool}
	productSvc.SetTaskSubmitter(productSubmitter)
	productProcessor.SetTaskSubmitter(productSubmitter)

	productHandler, err := productapi.NewProductHandler(productSvc)
	if err != nil {
		return nil, fmt.Errorf("create product handler: %w", err)
	}

	return &Module{
		Handler: productHandler,
		Pool:    productPool,
		Service: productSvc,
		Closers: closers,
	}, nil
}

func buildTaskRepository(cfg *config.Config, logger *logrus.Logger) (productenrich.TaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create product task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory productenrich repository")
	return productstore.NewMemTaskRepository(), nil, nil
}

func buildRedisClient(cfg *config.Config, logger *logrus.Logger) (productenrich.RedisClient, error) {
	if cfg != nil && cfg.Redis != nil && cfg.Redis.Host != "" {
		redisClient, err := redisclient.New(cfg.Redis)
		if err != nil {
			return nil, err
		}
		logger.Infof("Redis connected: %s:%d db=%d", cfg.Redis.Host, cfg.Redis.Port, cfg.Redis.DB)
		return redisClient, nil
	}

	logger.Warn("Redis not configured, using in-memory productenrich queue fallback")
	return productenrich.NewMemRedisClient(), nil
}

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productenrich.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	databaseConfig := platformDatabaseConfig(cfg)
	db, err := platformdatabase.OpenShared(databaseConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if config.ProductListingAPIRuntimeAutoMigrateEnabled() {
		if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
			return nil, nil, fmt.Errorf("database auto-migrate failed: %w", err)
		}
	}

	repo := productstore.NewTaskRepository(db)
	closer := func() error { return platformdatabase.CloseShared(databaseConfig, db) }
	return repo, closer, nil
}

func platformDatabaseConfig(cfg *config.DatabaseConfig) *platformdatabase.Config {
	if cfg == nil {
		return nil
	}
	return &platformdatabase.Config{
		Host:                  cfg.Host,
		Port:                  cfg.Port,
		User:                  cfg.User,
		Password:              cfg.Password,
		Database:              cfg.Database,
		MaxConnections:        cfg.MaxConnections,
		MaxIdleConnections:    cfg.MaxIdleConnections,
		ConnectionMaxLifetime: cfg.ConnectionMaxLifetime,
	}
}
