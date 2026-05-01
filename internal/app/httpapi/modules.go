package httpapi

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingapi "task-processor/internal/amazonlisting/api"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productpipeline "task-processor/internal/productenrich/pipeline"
	productstore "task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimageapi "task-processor/internal/productimage/api"
	productimagepipeline "task-processor/internal/productimage/pipeline"
	productimagestore "task-processor/internal/productimage/store"
	sheinpub "task-processor/internal/publishing/shein"
	sdsclient "task-processor/internal/sds/client"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/taskrpcapi"
)

var newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service) (sdsusecase.Service, *sdsclient.AuthState, error) {
	sdsHTTPClient, err := sdsclient.New(sdsclient.DefaultConfig())
	if err != nil {
		return nil, nil, err
	}

	authState := sdsHTTPClient.AuthState()
	if authState == nil || strings.TrimSpace(authState.AccessToken) == "" {
		return nil, authState, nil
	}

	svc, err := sdsusecase.NewService(sdsusecase.Config{
		SDSClient:    sdsHTTPClient,
		ImageService: imageSvc,
	})
	if err != nil {
		return nil, authState, err
	}

	return svc, authState, nil
}

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

	listingKitModule, err := buildListingKitModule(logger, deps)
	if err != nil {
		return nil, err
	}

	localTaskHealthProvider := buildLocalTaskHealthProvider(map[string]worker.WorkerPool{
		"product_enrich": productModule.pool,
		"product_image":  imageModule.pool,
		"amazon_listing": amazonListingModule.pool,
		"listing_kit":    listingKitModule.pool,
	})

	taskRPCHandler, err := buildTaskRPCModule(deps, localTaskHealthProvider)
	if err != nil {
		return nil, err
	}

	sdsCatalogHandler := buildSDSCatalogHandler(logger)

	server, routes := buildHTTPServerBundleWithStudio(options.Port, productModule.handler, imageModule.handler, amazonListingModule.handler, listingKitModule.handler, listingKitModule.studioSessionHandler, taskRPCHandler, sdsCatalogHandler)
	return &appBootstrap{
		productHandler:       productModule.handler,
		imageHandler:         imageModule.handler,
		amazonListingHandler: amazonListingModule.handler,
		listingKitHandler:    listingKitModule.handler,
		studioSessionHandler: listingKitModule.studioSessionHandler,
		sdsCatalogHandler:    sdsCatalogHandler,
		taskRPCHandler:       taskRPCHandler,
		server:               server,
		routes:               routes,
		pools:                []worker.WorkerPool{productModule.pool, imageModule.pool, amazonListingModule.pool, listingKitModule.pool},
		closers:              deps.closers,
	}, nil
}

func buildSDSCatalogHandler(logger *logrus.Logger) sdsCatalogRouteHandler {
	sdsHTTPClient, err := sdsclient.New(sdsclient.DefaultConfig())
	if err != nil {
		logger.WithError(err).Warn("failed to initialize SDS catalog client")
		return newSDSCatalogHandler(nil)
	}
	return newSDSCatalogHandler(sdstemplate.NewService(sdsHTTPClient))
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
	scoreCache := productenrich.NewLLMScoreCache(redisClient, nil)

	llmScorer := buildProductLLMScorerWithCache(deps.cfg, deps.llmMgr, scoreCache)
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
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(deps.imageWorkDir, deps.cfg.Watermark, logger)
	if err != nil {
		return nil, fmt.Errorf("create watermark-aware image cleaner: %w", err)
	}
	modelProvider, err := buildProductImageModelProvider(deps.cfg, deps.llmMgr, deps.openaiMgr, deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create productimage model provider: %w", err)
	}

	var subjectExtractor productimage.SubjectExtractor
	var whiteBgRenderer productimage.WhiteBackgroundRenderer
	var sceneRenderer productimage.SceneRenderer
	if !shouldUseModelBackedImagePipeline(modelProvider) || modelProvider.FaithfulEditor() == nil {
		subjectExtractor, err = buildImageSubjectExtractor(deps.cfg, deps.imageWorkDir)
		if err != nil {
			return nil, fmt.Errorf("create subject extractor: %w", err)
		}
		whiteBgRenderer, err = buildWhiteBackgroundRenderer(deps.cfg, deps.imageWorkDir)
		if err != nil {
			return nil, fmt.Errorf("create white background renderer: %w", err)
		}
	}
	if modelProvider == nil || modelProvider.SceneGenerator() == nil {
		if !shouldUseModelBackedImagePipeline(modelProvider) {
			sceneRenderer, err = buildSceneRenderer(deps.cfg, deps.imageWorkDir)
			if err != nil {
				return nil, fmt.Errorf("create scene renderer: %w", err)
			}
		}
	}
	resolvedComponents := resolveImagePipelineComponents(modelProvider, subjectExtractor, whiteBgRenderer, sceneRenderer)
	subjectExtractor = resolvedComponents.subjectExtractor
	whiteBgRenderer = resolvedComponents.whiteBgRenderer
	sceneRenderer = resolvedComponents.sceneRenderer

	imageCapabilities := productimage.StrictServiceCapabilities()
	imageSvc, err := productimage.NewService(&productimage.ServiceConfig{
		QueueName:             "product_image_tasks",
		TaskRepo:              imageRepo,
		ModelProvider:         modelProvider,
		Capabilities:          &imageCapabilities,
		SourceParser:          sourceParser,
		ContextAnalyzer:       contextAnalyzer,
		ImageInspector:        imageInspector,
		ImageRanker:           productimage.NewDefaultImageRanker(),
		SubjectExtractor:      subjectExtractor,
		ImageCleaner:          imageCleaner,
		WhiteBgRenderer:       whiteBgRenderer,
		SceneRenderer:         sceneRenderer,
		AssetPublisher:        buildImageAssetPublisher(deps.cfg, logger),
		CleanupTemporaryFiles: deps.cfg.ProductImage.Lifecycle.CleanupTemporaryFiles,
		ReuseExistingAssets:   deps.cfg.ProductImage.Lifecycle.ReuseExistingAssets,
	})
	if err != nil {
		return nil, fmt.Errorf("create image service: %w", err)
	}

	deps.imageService = imageSvc
	deps.imageSubjectExtractor = subjectExtractor
	deps.imageWhiteBgRenderer = whiteBgRenderer
	deps.imageSceneRenderer = sceneRenderer

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

func buildSceneRenderer(_ *config.Config, imageWorkDir string) (productimage.SceneRenderer, error) {
	return productimage.NewDefaultSceneRenderer(imageWorkDir)
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
	case "s3":
		return buildProductImageS3AssetPublisher(cfg, logger)
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

func buildListingKitModule(logger *logrus.Logger, deps *runtimeDeps) (*listingKitModule, error) {
	repo, closers, err := buildListingKitTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, closers...)

	assetRepository, assetClosers, err := buildAssetRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, assetClosers...)

	reviewRepository, reviewClosers, err := buildListingKitReviewRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, reviewClosers...)

	studioSessionRepository, studioSessionClosers, err := buildListingKitStudioSessionRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, studioSessionClosers...)

	resolutionCacheStore, resolutionCacheClosers, err := buildSheinResolutionCacheStore(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, resolutionCacheClosers...)

	sheinCategoryResolver := sheinpub.NewCachedCategoryResolver(sheinpub.NewManagedCategoryResolver(deps.managementClient, buildSheinCategoryLLMClient(deps.openaiMgr)), resolutionCacheStore)
	sheinAttributeResolver := sheinpub.NewCachedAttributeResolver(sheinpub.NewManagedAttributeResolver(deps.managementClient, buildSheinSaleAttributeLLMClient(deps.cfg, deps.openaiMgr)), resolutionCacheStore)
	sheinSaleAttributeResolver := sheinpub.NewCachedSaleAttributeResolver(sheinpub.NewManagedSaleAttributeResolver(deps.managementClient, buildSheinSaleAttributeLLMClient(deps.cfg, deps.openaiMgr)), resolutionCacheStore)
	sheinProductAPIBuilder := sheinpub.NewManagedProductAPIBuilder(deps.managementClient)
	sheinImageAPIBuilder := sheinpub.NewManagedImageAPIBuilder(deps.managementClient)
	sheinTranslateAPIBuilder := sheinpub.NewManagedTranslateAPIBuilder(deps.managementClient)
	sheinPricingPolicy := buildListingKitSheinPricingPolicy(deps.cfg)
	deps.sdsSyncService = buildSDSSyncService(logger, deps)

	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Repository:              repo,
		StudioSessionRepository: studioSessionRepository,
		ProductService:          deps.productService,
		ImageService:            deps.imageService,
		SDSSyncService:          deps.sdsSyncService,
		SheinDefaultStoreID:     resolveListingKitDefaultSheinStoreID(deps.cfg.Management.StoreIDs),
		ImageUploadStore:        buildListingKitImageUploadStore(deps.cfg, logger),
		AssetRepository:         assetRepository,
		ReviewRepository:        reviewRepository,
		AssetRecipeResolver:     assetrecipe.NewStaticResolver(),
		AssetBundleBuilder:      assetbundle.NewBuilder(),
		AssetGenerationService: assetgeneration.NewService(assetgeneration.Config{
			SubjectExtractor:        deps.imageSubjectExtractor,
			WhiteBackgroundRenderer: deps.imageWhiteBgRenderer,
			DeferredRenderer:        assetgeneration.NewProductImageDeferredRenderer(deps.imageSceneRenderer),
		}),
		SheinManagementClient:      deps.managementClient,
		SheinCategoryResolver:      sheinCategoryResolver,
		SheinAttributeResolver:     sheinAttributeResolver,
		SheinSaleAttributeResolver: sheinSaleAttributeResolver,
		SheinPricingPolicy:         sheinPricingPolicy,
		SheinProductAPIBuilder:     sheinProductAPIBuilder,
		SheinImageAPIBuilder:       sheinImageAPIBuilder,
		SheinTranslateAPIBuilder:   sheinTranslateAPIBuilder,
		SheinContentOptimizer:      buildSheinCategoryLLMClient(deps.openaiMgr),
		StudioImageGenerator:       buildStudioImageGenerator(deps.cfg, deps.openaiMgr),
		Assembler: listingkit.NewAssemblerWithConfig(listingkit.AssemblerConfig{
			SheinCategoryResolver:      sheinCategoryResolver,
			SheinAttributeResolver:     sheinAttributeResolver,
			SheinSaleAttributeResolver: sheinSaleAttributeResolver,
			SheinPricingPolicy:         sheinPricingPolicy,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("create listing kit service: %w", err)
	}

	processor, err := listingkit.NewProcessor(svc, repo, logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create listing kit processor: %w", err)
	}
	pool := newWorkerPool(processor, deps.cfg)
	submitter := &poolSubmitter{pool: pool}
	svc.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)

	handler, err := listingkitapi.NewHandler(svc)
	if err != nil {
		return nil, fmt.Errorf("create listing kit handler: %w", err)
	}
	studioSessionService, ok := svc.(listingkit.StudioSessionHandlerService)
	if !ok {
		return nil, fmt.Errorf("listing kit service does not implement studio session handler service")
	}
	studioSessionHandler, err := listingkitapi.NewStudioSessionHandler(studioSessionService)
	if err != nil {
		return nil, fmt.Errorf("create listing kit studio session handler: %w", err)
	}
	return &listingKitModule{handler: handler, studioSessionHandler: studioSessionHandler, pool: pool}, nil
}

func buildListingKitSheinPricingPolicy(cfg *config.Config) sheinpub.PricingPolicy {
	if cfg == nil {
		return sheinpub.PricingPolicy{}
	}
	pricing := cfg.Platforms.Shein.ListingPricing
	return sheinpub.PricingPolicy{
		Enabled:        pricing.Enabled,
		Currency:       pricing.Currency,
		MarkupRate:     pricing.MarkupRate,
		FixedMarkup:    pricing.FixedMarkup,
		ShippingCost:   pricing.ShippingCost,
		CommissionRate: pricing.CommissionRate,
		MinimumPrice:   pricing.MinimumPrice,
		RoundTo:        pricing.RoundTo,
	}
}

func buildListingKitImageUploadStore(cfg *config.Config, logger *logrus.Logger) listingkit.ImageUploadStore {
	if cfg == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(cfg.ProductImage.Publisher.Provider), "s3") {
		return buildListingKitS3ImageUploadStore(cfg, logger)
	}
	rootDir := filepath.Join(cfg.ProductImage.Publisher.OutputDir, "listingkit-inputs")
	store, err := listingkit.NewLocalImageUploadStore(rootDir)
	if err != nil {
		return nil
	}
	return store
}

func buildListingKitReviewRepository(cfg *config.Config, logger *logrus.Logger) (reviewstore.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitReviewRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit review repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit review repository")
	return reviewstore.NewMemRepository(), nil, nil
}

func buildListingKitStudioSessionRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StudioSessionRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStudioSessionRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit studio session repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, SHEIN studio session repository disabled")
	return nil, nil, nil
}

func buildSheinResolutionCacheStore(cfg *config.Config, logger *logrus.Logger) (sheinpub.ResolutionCacheStore, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		store, closer, err := newDBSheinResolutionCacheStore(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create shein resolution cache store: %w", err)
		}
		return store, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory SHEIN resolution cache fallback")
	return nil, nil, nil
}

func buildAssetRepository(cfg *config.Config, logger *logrus.Logger) (assetrepo.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBAssetRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create asset repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory asset repository")
	return assetrepo.NewMemRepository(), nil, nil
}

func buildListingKitTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit repository")
	return listingkitstore.NewMemTaskRepository(), nil, nil
}

func buildSDSSyncService(logger *logrus.Logger, deps *runtimeDeps) sdsusecase.Service {
	if deps == nil || deps.imageService == nil {
		return nil
	}

	svc, authState, err := newSDSSyncServiceForHTTPAPI(deps.imageService)
	if err != nil {
		logger.WithError(err).Warn("failed to initialize SDS client; SDS sync disabled")
		return nil
	}

	if authState == nil || strings.TrimSpace(authState.AccessToken) == "" {
		logger.Info("SDS auth state not found; SDS sync disabled")
		return nil
	}

	return svc
}

func buildTaskRPCModule(deps *runtimeDeps, localStatusProvider taskrpcapi.LocalStatusProvider) (taskrpcapi.Handler, error) {
	if deps == nil || deps.managementClient == nil {
		return nil, nil
	}

	return taskrpcapi.NewHandler(deps.managementClient.GetTaskRPCClient(), localStatusProvider)
}

func buildLocalTaskHealthProvider(pools map[string]worker.WorkerPool) taskrpcapi.LocalStatusProvider {
	return func() map[string]any {
		summary := map[string]any{
			"poolCount":           0,
			"totalQueueSize":      0,
			"totalBufferSize":     0,
			"totalAvailableSlots": 0,
			"totalSubmitted":      int64(0),
			"totalProcessed":      int64(0),
			"totalSucceeded":      int64(0),
			"totalFailed":         int64(0),
			"totalPanicked":       int64(0),
			"queueFullCount":      int64(0),
		}
		poolSnapshots := make(map[string]any, len(pools))

		for name, pool := range pools {
			if pool == nil {
				continue
			}

			queueStats := pool.GetQueueStats()
			poolSnapshot := map[string]any{
				"queueSize":      queueStats.QueueSize,
				"bufferSize":     queueStats.BufferSize,
				"availableSlots": queueStats.AvailableSlots,
				"usagePercent":   queueStats.UsagePercent,
			}

			summary["poolCount"] = summary["poolCount"].(int) + 1
			summary["totalQueueSize"] = summary["totalQueueSize"].(int) + queueStats.QueueSize
			summary["totalBufferSize"] = summary["totalBufferSize"].(int) + queueStats.BufferSize
			summary["totalAvailableSlots"] = summary["totalAvailableSlots"].(int) + queueStats.AvailableSlots

			if metrics := pool.GetMetrics(); metrics != nil {
				snapshot := metrics.GetSnapshot()
				poolSnapshot["metrics"] = map[string]any{
					"totalSubmitted": snapshot.TotalSubmitted,
					"totalProcessed": snapshot.TotalProcessed,
					"totalSucceeded": snapshot.TotalSucceeded,
					"totalFailed":    snapshot.TotalFailed,
					"totalPanicked":  snapshot.TotalPanicked,
					"queueFullCount": snapshot.QueueFullCount,
					"successRate":    snapshot.SuccessRate(),
					"failureRate":    snapshot.FailureRate(),
					"panicRate":      snapshot.PanicRate(),
					"uptimeSeconds":  int64(snapshot.Uptime.Seconds()),
				}
				summary["totalSubmitted"] = summary["totalSubmitted"].(int64) + snapshot.TotalSubmitted
				summary["totalProcessed"] = summary["totalProcessed"].(int64) + snapshot.TotalProcessed
				summary["totalSucceeded"] = summary["totalSucceeded"].(int64) + snapshot.TotalSucceeded
				summary["totalFailed"] = summary["totalFailed"].(int64) + snapshot.TotalFailed
				summary["totalPanicked"] = summary["totalPanicked"].(int64) + snapshot.TotalPanicked
				summary["queueFullCount"] = summary["queueFullCount"].(int64) + snapshot.QueueFullCount
			}

			poolSnapshots[name] = poolSnapshot
		}

		return map[string]any{
			"summary": summary,
			"pools":   poolSnapshots,
		}
	}
}
