package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/appenv"
	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productpipeline "task-processor/internal/productenrich/pipeline"
	"task-processor/internal/productenrich/store"
	"task-processor/internal/productimage"
	productimageapi "task-processor/internal/productimage/api"
	productimagepipeline "task-processor/internal/productimage/pipeline"
	productimagestore "task-processor/internal/productimage/store"
	"task-processor/internal/prompt"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "config file path")
	logLevel   = flag.String("log-level", "info", "log level")
	port       = flag.Int("port", 8085, "API service port")
)

var (
	appVersion = "1.0.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	logger := appenv.SetupLoggerWithLevel(*logLevel)
	appenv.PrintVersionInfo(logger, appenv.VersionInfo{
		Version:   appVersion,
		BuildTime: buildTime,
	})

	logger.Info("starting productenrich API service")
	logger.Infof("config path: %s", *configPath)
	logger.Infof("API port: %d", *port)

	if err := run(logger); err != nil {
		logger.Fatalf("service startup failed: %v", err)
	}
}

func run(logger *logrus.Logger) error {
	productHandler, imageHandler, pools, closers, err := buildHandlers(logger)
	if err != nil {
		return fmt.Errorf("build handlers: %w", err)
	}
	defer func() {
		for _, closeFn := range closers {
			if closeFn == nil {
				continue
			}
			if closeErr := closeFn(); closeErr != nil {
				logger.Warnf("close resource failed: %v", closeErr)
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, pool := range pools {
		pool.Start(ctx)
	}
	logger.Infof("worker pools started: %d", len(pools))

	router := gin.New()
	router.Use(gin.Recovery())
	registerRoutes(router, productHandler, imageHandler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: router,
	}

	go func() {
		logger.Infof("API service listening on port %d", *port)
		logger.Info("endpoints:")
		logger.Info("  - POST /api/v1/products/generate")
		logger.Info("  - GET  /api/v1/products/tasks/:task_id")
		logger.Info("  - POST /api/v1/images/process")
		logger.Info("  - GET  /api/v1/images/tasks/:task_id")
		logger.Info("  - POST /api/v1/images/tasks/:task_id/review")
		logger.Info("  - GET  /health")
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			logger.Fatalf("HTTP service exited unexpectedly: %v", listenErr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	sig := <-sigChan
	logger.Infof("received signal %v, shutting down", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	cancel()
	for _, pool := range pools {
		pool.Stop(shutdownCtx)
	}
	logger.Info("service shut down gracefully")
	return nil
}

func buildHandlers(logger *logrus.Logger) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	cfg := config.LoadConfigFromFile(*configPath)
	var closers []func() error
	imageWorkDir := resolveImageWorkDir(cfg)

	promptsDir := cfg.Prompts.Dir
	if promptsDir == "" {
		promptsDir = "./prompts"
	}
	if err := prompt.InitGlobal(context.Background(), promptsDir, cfg.Prompts.HotReload, logger.WithField("component", "prompt")); err != nil {
		logger.Warnf("prompt registry init failed, using fallback prompts: %v", err)
	}

	llmMgr, err := newLLMManager(cfg.OpenAI)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create llm manager: %w", err)
	}

	productUnderstanding, err := productenrichenrich.NewProductUnderstanding(llmMgr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create product understanding: %w", err)
	}

	jsonGenerator, err := productenrichenrich.NewJSONGenerator(logger, llmMgr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create JSON generator: %w", err)
	}

	variantGenerator, err := productenrichenrich.NewVariantGenerator(llmMgr)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("create variant generator: %w", err)
	}

	llmScorer := productenrich.NewLLMScorer(&productenrich.LLMScorerConfig{LLMManager: llmMgr})
	qualityScorer := productenrich.NewQualityScorer(&productenrich.QualityScorerConfig{
		ImageWeight:   0.4,
		TextWeight:    0.3,
		ScrapedWeight: 0.3,
		LLMScorer:     llmScorer,
		EnableLLM:     true,
	})
	strategySelector := productenrich.NewStrategySelector(nil)
	resultValidator := productenrich.NewResultValidator()
	enhancementSuggester := productenrich.NewEnhancementSuggester()
	inputValidator := productenrich.NewInputValidator(&productenrich.InputValidatorConfig{
		HTTPTimeout: 5 * time.Second,
		MaxWorkers:  10,
	})

	var taskRepo productenrich.TaskRepository
	if cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, repoErr := newDBTaskRepository(cfg.Database, logger)
		if repoErr != nil {
			return nil, nil, nil, closers, fmt.Errorf("create task repository: %w", repoErr)
		}
		taskRepo = repo
		closers = append(closers, closer)
	} else {
		logger.Warn("database not configured, using in-memory productenrich repository")
		taskRepo = store.NewMemTaskRepository()
	}

	var redisC productenrich.RedisClient
	if cfg.Redis != nil && cfg.Redis.Host != "" {
		rc, redisErr := newRedisClient(cfg.Redis, logger)
		if redisErr != nil {
			return nil, nil, nil, closers, fmt.Errorf("create Redis client: %w", redisErr)
		}
		redisC = rc
	} else {
		logger.Warn("redis not configured, using in-memory productenrich queue fallback")
		redisC = productenrich.NewMemRedisClient()
	}

	webScraper := newWebScraper(cfg)
	inputParser, err := productenrichenrich.NewInputParser(logger, &productenrich.InputParserConfig{}, webScraper)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create input parser: %w", err)
	}

	productCapabilities := productenrich.StrictProductServiceCapabilities()
	productSvc, err := productenrich.NewProductService(&productenrich.ProductServiceConfig{
		QueueName:            "product_enrich_tasks",
		TaskRepo:             taskRepo,
		RedisClient:          redisC,
		Capabilities:         &productCapabilities,
		InputParser:          inputParser,
		ProductUnderstanding: productUnderstanding,
		JSONGenerator:        jsonGenerator,
		VariantGenerator:     variantGenerator,
		QualityScorer:        qualityScorer,
		StrategySelector:     strategySelector,
		ResultValidator:      resultValidator,
		EnhancementSuggester: enhancementSuggester,
		InputValidator:       inputValidator,
	})
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create product service: %w", err)
	}

	productProcessor, err := productpipeline.NewProcessor(productSvc, taskRepo, logger, 3)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create product processor: %w", err)
	}
	productPool := worker.NewPoolWithConfig(productProcessor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
	productSubmitter := &poolSubmitter{pool: productPool}
	productSvc.SetTaskSubmitter(productSubmitter)
	productProcessor.SetTaskSubmitter(productSubmitter)

	productHandler, err := productapi.NewProductHandler(productSvc)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create product handler: %w", err)
	}

	sourceParser, err := productimage.NewSourceParser(inputParser)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create image source parser: %w", err)
	}
	contextAnalyzer, err := productimage.NewProductContextAnalyzer(productUnderstanding)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create image context analyzer: %w", err)
	}

	var imageRepo productimage.TaskRepository
	if cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, repoErr := newDBImageTaskRepository(cfg.Database, logger)
		if repoErr != nil {
			return nil, nil, nil, closers, fmt.Errorf("create image task repository: %w", repoErr)
		}
		imageRepo = repo
		closers = append(closers, closer)
	} else {
		logger.Warn("database not configured, using in-memory productimage repository")
		imageRepo = productimagestore.NewMemTaskRepository()
	}
	imageCapabilities := productimage.StrictServiceCapabilities()
	imageInspector, err := productimage.NewDownloadedImageInspector(imageWorkDir)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create downloaded image inspector: %w", err)
	}
	subjectExtractor, err := buildImageSubjectExtractor(cfg, imageWorkDir)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create subject extractor: %w", err)
	}
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(imageWorkDir, cfg.Watermark, logger)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create downloaded image cleaner: %w", err)
	}
	whiteBgRenderer, err := buildWhiteBackgroundRenderer(cfg, imageWorkDir)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create white background renderer: %w", err)
	}
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
		AssetPublisher:        buildImageAssetPublisher(cfg, logger),
		CleanupTemporaryFiles: cfg.ProductImage.Lifecycle.CleanupTemporaryFiles,
		ReuseExistingAssets:   cfg.ProductImage.Lifecycle.ReuseExistingAssets,
	})
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create image service: %w", err)
	}

	imageProcessor, err := productimagepipeline.NewProcessor(imageSvc, imageRepo, logger, 2)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create image processor: %w", err)
	}
	imagePool := worker.NewPoolWithConfig(imageProcessor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
	imageSubmitter := &poolSubmitter{pool: imagePool}
	imageSvc.SetTaskSubmitter(imageSubmitter)
	imageProcessor.SetTaskSubmitter(imageSubmitter)

	imageHandler, err := productimageapi.NewImageHandler(imageSvc)
	if err != nil {
		return nil, nil, nil, closers, fmt.Errorf("create image handler: %w", err)
	}

	return productHandler, imageHandler, []worker.WorkerPool{productPool, imagePool}, closers, nil
}

func resolveImageWorkDir(cfg *config.Config) string {
	if cfg == nil {
		return filepath.Join(".", "tmp", "productimage")
	}
	workDir := filepath.Clean(cfg.ProductImage.WorkDir)
	if workDir == "" || workDir == "." {
		return filepath.Join(".", "tmp", "productimage")
	}
	return workDir
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
			logger.WithError(err).Warn("productimage local asset publisher is disabled")
			return nil
		}
		return publisher
	case "amazon":
		publisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("productimage amazon asset publisher is disabled")
			return nil
		}
		return publisher
	case "hybrid":
		localPublisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("productimage hybrid local asset publisher is disabled")
			return nil
		}
		amazonPublisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("productimage hybrid amazon asset publisher is partially disabled")
			return localPublisher
		}
		return productimage.NewMultiAssetPublisher(localPublisher, amazonPublisher)
	default:
		logger.Warnf("unsupported productimage publisher provider: %s", provider)
		return nil
	}
}

func registerRoutes(r *gin.Engine, productHandler productenrich.ProductHandler, imageHandler productimage.Handler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if productHandler != nil {
		v1 := r.Group("/api/v1/products")
		{
			v1.POST("/generate", productHandler.GenerateProduct)
			v1.GET("/tasks/:task_id", productHandler.GetTaskResult)
		}
	}

	if imageHandler != nil {
		v1 := r.Group("/api/v1/images")
		{
			v1.POST("/process", imageHandler.ProcessImages)
			v1.GET("/tasks/:task_id", imageHandler.GetTaskResult)
			v1.POST("/tasks/:task_id/review", imageHandler.ReviewTask)
		}
	}
}
