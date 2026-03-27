package httpapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingapi "task-processor/internal/amazonlisting/api"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productpipeline "task-processor/internal/productenrich/pipeline"
	productstore "task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimageapi "task-processor/internal/productimage/api"
	productimagepipeline "task-processor/internal/productimage/pipeline"
	productimagestore "task-processor/internal/productimage/store"
)

func buildBootstrap(logger *logrus.Logger, options Options) (*appBootstrap, error) {
	deps, err := buildRuntimeDeps(logger, options.ConfigPath)
	if err != nil {
		return nil, err
	}

	productModule, err := buildProductModule(logger, deps)
	if err != nil {
		return nil, err
	}

	imageModule, err := buildImageModule(logger, deps)
	if err != nil {
		return nil, err
	}

	amazonListingModule, err := buildAmazonListingModule(logger, deps)
	if err != nil {
		return nil, err
	}

	server := buildHTTPServer(options.Port, productModule.handler, imageModule.handler, amazonListingModule.handler)
	return &appBootstrap{
		productHandler:       productModule.handler,
		imageHandler:         imageModule.handler,
		amazonListingHandler: amazonListingModule.handler,
		server:               server,
		pools:                []worker.WorkerPool{productModule.pool, imageModule.pool, amazonListingModule.pool},
		closers:              deps.closers,
	}, nil
}

func BuildHandlers(logger *logrus.Logger, options Options) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	bootstrap, err := buildBootstrap(logger, options)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return bootstrap.productHandler, bootstrap.imageHandler, bootstrap.pools, bootstrap.closers, nil
}

func newWorkerPool(processor worker.Processor, cfg *config.Config) worker.WorkerPool {
	return worker.NewPoolWithConfig(processor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
}

func buildProductModule(logger *logrus.Logger, deps *runtimeDeps) (*productModule, error) {
	jsonGenerator, err := productenrichenrich.NewJSONGenerator(logger, deps.llmMgr)
	if err != nil {
		return nil, fmt.Errorf("create JSON generator: %w", err)
	}

	variantGenerator, err := productenrichenrich.NewVariantGenerator(deps.llmMgr)
	if err != nil {
		return nil, fmt.Errorf("create variant generator: %w", err)
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
		return nil, fmt.Errorf("create product service: %w", err)
	}

	deps.productService = productSvc

	productProcessor, err := productpipeline.NewProcessor(productSvc, taskRepo, logger, 3)
	if err != nil {
		return nil, fmt.Errorf("create product processor: %w", err)
	}
	productPool := newWorkerPool(productProcessor, deps.cfg)
	productSubmitter := &poolSubmitter{pool: productPool}
	productSvc.SetTaskSubmitter(productSubmitter)
	productProcessor.SetTaskSubmitter(productSubmitter)

	productHandler, err := productapi.NewProductHandler(productSvc)
	if err != nil {
		return nil, fmt.Errorf("create product handler: %w", err)
	}

	return &productModule{handler: productHandler, pool: productPool}, nil
}

func buildProductTaskRepository(cfg *config.Config, logger *logrus.Logger) (productenrich.TaskRepository, []func() error, error) {
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

func buildProductRedisClient(cfg *config.Config, logger *logrus.Logger) (productenrich.RedisClient, error) {
	if cfg != nil && cfg.Redis != nil && cfg.Redis.Host != "" {
		redisClient, err := newRedisClient(cfg.Redis, logger)
		if err != nil {
			return nil, fmt.Errorf("create Redis client: %w", err)
		}
		return redisClient, nil
	}

	logger.Warn("Redis not configured, using in-memory productenrich queue fallback")
	return productenrich.NewMemRedisClient(), nil
}

func buildImageModule(logger *logrus.Logger, deps *runtimeDeps) (*imageModule, error) {
	sourceParser, err := productimage.NewSourceParser(deps.inputParser)
	if err != nil {
		return nil, fmt.Errorf("create image source parser: %w", err)
	}
	contextAnalyzer, err := productimage.NewProductContextAnalyzer(deps.understanding)
	if err != nil {
		return nil, fmt.Errorf("create image context analyzer: %w", err)
	}

	imageRepo, closers, err := buildImageTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, closers...)

	imageInspector, err := productimage.NewDownloadedImageInspector(deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create downloaded image inspector: %w", err)
	}
	subjectExtractor, err := buildImageSubjectExtractor(deps.cfg, deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create subject extractor: %w", err)
	}
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(deps.imageWorkDir, deps.cfg.Watermark, logger)
	if err != nil {
		return nil, fmt.Errorf("create watermark-aware image cleaner: %w", err)
	}
	whiteBgRenderer, err := buildWhiteBackgroundRenderer(deps.cfg, deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create white background renderer: %w", err)
	}

	imageCapabilities := productimage.StrictServiceCapabilities()
	imageSvc, err := productimage.NewService(&productimage.ServiceConfig{
		QueueName:             "product_image_tasks",
		TaskRepo:              imageRepo,
		Capabilities:          &imageCapabilities,
		SourceParser:          sourceParser,
		ContextAnalyzer:       contextAnalyzer,
		ImageInspector:        imageInspector,
		ImageRanker:           productimage.NewDefaultImageRanker(),
		SubjectExtractor:      subjectExtractor,
		ImageCleaner:          imageCleaner,
		WhiteBgRenderer:       whiteBgRenderer,
		AssetPublisher:        buildImageAssetPublisher(deps.cfg, logger),
		CleanupTemporaryFiles: deps.cfg.ProductImage.Lifecycle.CleanupTemporaryFiles,
		ReuseExistingAssets:   deps.cfg.ProductImage.Lifecycle.ReuseExistingAssets,
	})
	if err != nil {
		return nil, fmt.Errorf("create image service: %w", err)
	}

	deps.imageService = imageSvc

	imageProcessor, err := productimagepipeline.NewProcessor(imageSvc, imageRepo, logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create image processor: %w", err)
	}
	imagePool := newWorkerPool(imageProcessor, deps.cfg)
	imageSubmitter := &poolSubmitter{pool: imagePool}
	imageSvc.SetTaskSubmitter(imageSubmitter)
	imageProcessor.SetTaskSubmitter(imageSubmitter)

	imageHandler, err := productimageapi.NewImageHandler(imageSvc)
	if err != nil {
		return nil, fmt.Errorf("create image handler: %w", err)
	}

	return &imageModule{handler: imageHandler, pool: imagePool}, nil
}

func buildImageTaskRepository(cfg *config.Config, logger *logrus.Logger) (productimage.TaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBImageTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create image task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory productimage repository")
	return productimagestore.NewMemTaskRepository(), nil, nil
}

func buildImageSubjectExtractor(cfg *config.Config, imageWorkDir string) (productimage.SubjectExtractor, error) {
	if cfg == nil || !cfg.ProductImage.Segmenter.Enabled || cfg.ProductImage.Segmenter.Endpoint == "" {
		return productimage.NewHybridSubjectExtractor(imageWorkDir, nil)
	}

	client, err := productimage.NewHTTPSegmentationClient(productimage.HTTPSegmentationClientConfig{
		Endpoint: cfg.ProductImage.Segmenter.Endpoint,
		APIKey:   cfg.ProductImage.Segmenter.APIKey,
		Timeout:  time.Duration(cfg.ProductImage.Segmenter.Timeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return productimage.NewHybridSubjectExtractor(imageWorkDir, client)
}

func buildWhiteBackgroundRenderer(cfg *config.Config, imageWorkDir string) (productimage.WhiteBackgroundRenderer, error) {
	if cfg == nil || !cfg.ProductImage.WhiteBackground.Enabled || cfg.ProductImage.WhiteBackground.Endpoint == "" {
		return productimage.NewHybridWhiteBackgroundRenderer(imageWorkDir, nil)
	}

	client, err := productimage.NewHTTPWhiteBackgroundClient(productimage.HTTPWhiteBackgroundClientConfig{
		Endpoint: cfg.ProductImage.WhiteBackground.Endpoint,
		APIKey:   cfg.ProductImage.WhiteBackground.APIKey,
		Timeout:  time.Duration(cfg.ProductImage.WhiteBackground.Timeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return productimage.NewHybridWhiteBackgroundRenderer(imageWorkDir, client)
}

func buildImageAssetPublisher(cfg *config.Config, logger *logrus.Logger) productimage.AssetPublisher {
	if cfg == nil || !cfg.ProductImage.Publisher.Enabled {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.ProductImage.Publisher.Provider))
	switch provider {
	case "", "local":
		publisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("local image asset publisher unavailable")
			return nil
		}
		return publisher
	case "amazon":
		publisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("amazon image asset publisher unavailable")
			return nil
		}
		return publisher
	case "hybrid":
		localPublisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("hybrid local image asset publisher unavailable")
			return nil
		}
		amazonPublisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("hybrid amazon image asset publisher partially unavailable")
			return localPublisher
		}
		return productimage.NewMultiAssetPublisher(localPublisher, amazonPublisher)
	default:
		logger.Warnf("unsupported image publisher provider: %s", provider)
		return nil
	}
}

func buildAmazonListingModule(logger *logrus.Logger, deps *runtimeDeps) (*amazonListingModule, error) {
	repo, closers, err := buildAmazonListingTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, closers...)

	svc, err := amazonlisting.NewService(&amazonlisting.ServiceConfig{
		Repository:       repo,
		ProductService:   deps.productService,
		ImageService:     deps.imageService,
		Assembler:        amazonlisting.NewAssembler(),
		ListingSubmitter: amazonlisting.NewSPAPISubmitter(deps.cfg),
		Validator:        amazonlisting.NewValidator(),
	})
	if err != nil {
		return nil, fmt.Errorf("create amazon listing service: %w", err)
	}

	processor, err := amazonlisting.NewProcessor(svc, repo, logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create amazon listing processor: %w", err)
	}
	pool := newWorkerPool(processor, deps.cfg)
	submitter := &poolSubmitter{pool: pool}
	svc.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)

	handler, err := amazonlistingapi.NewHandler(svc)
	if err != nil {
		return nil, fmt.Errorf("create amazon listing handler: %w", err)
	}

	return &amazonListingModule{handler: handler, pool: pool}, nil
}

func buildAmazonListingTaskRepository(cfg *config.Config, logger *logrus.Logger) (amazonlisting.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBAmazonListingTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create amazon listing task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory amazonlisting repository")
	return amazonlistingstore.NewMemTaskRepository(), nil, nil
}
