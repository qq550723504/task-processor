package httpapi

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/amazonlisting"
	amazonlistingapi "task-processor/internal/amazonlisting/api"
	amazonlistingstore "task-processor/internal/amazonlisting/store"
	appruntime "task-processor/internal/app/runtime"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	listingkitapi "task-processor/internal/listingkit/api"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/productenrich"
	productapi "task-processor/internal/productenrich/api"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	productpipeline "task-processor/internal/productenrich/pipeline"
	productstore "task-processor/internal/productenrich/store"
	productimage "task-processor/internal/productimage"
	productimageapi "task-processor/internal/productimage/api"
	productimagepipeline "task-processor/internal/productimage/pipeline"
	productimagestore "task-processor/internal/productimage/store"
	"task-processor/internal/promptmgmt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sheinpub "task-processor/internal/publishing/shein"
	sdsclient "task-processor/internal/sds/client"
	sdstemplate "task-processor/internal/sds/template"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sdslogin"
	sheinclient "task-processor/internal/shein/client"
	"task-processor/internal/sheinlogin"
	"task-processor/internal/taskrpcapi"
)

var newSDSSyncServiceForHTTPAPI = func(imageSvc productimage.Service, cfg *sdsclient.Config) (sdsusecase.Service, *sdsclient.AuthState, error) {
	if cfg == nil {
		cfg = sdsclient.DefaultConfig()
	}
	sdsHTTPClient, err := sdsclient.New(cfg)
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
	configureSheinLoginService(deps.cfg)

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

	sheinLoginHandler, sheinLoginCloser, err := buildSheinLoginModule(deps)
	if err != nil {
		return nil, err
	}
	if sheinLoginCloser != nil {
		deps.closers = append(deps.closers, sheinLoginCloser)
	}
	sdsLoginHandler, sdsLoginCloser, err := buildSDSLoginModule(deps)
	if err != nil {
		return nil, err
	}
	if sdsLoginCloser != nil {
		deps.closers = append(deps.closers, sdsLoginCloser)
	}
	listingKitModule, err := buildListingKitModule(logger, deps)
	if err != nil {
		return nil, err
	}
	promptTemplateModule := buildPromptTemplateModule(deps)

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

	sdsCatalogHandler := buildSDSCatalogHandler(logger, deps.cfg)

	server, routes := buildHTTPServerBundleWithStudio(options.Port, productModule.handler, imageModule.handler, amazonListingModule.handler, listingKitModule.handler, promptTemplateModule.handler, listingKitModule.studioSessionHandler, sheinLoginHandler, sdsLoginHandler, taskRPCHandler, sdsCatalogHandler)
	return &appBootstrap{
		productHandler:        productModule.handler,
		imageHandler:          imageModule.handler,
		amazonListingHandler:  amazonListingModule.handler,
		listingKitHandler:     listingKitModule.handler,
		promptTemplateHandler: promptTemplateModule.handler,
		studioSessionHandler:  listingKitModule.studioSessionHandler,
		sdsCatalogHandler:     sdsCatalogHandler,
		sheinLoginHandler:     sheinLoginHandler,
		sdsLoginHandler:       sdsLoginHandler,
		taskRPCHandler:        taskRPCHandler,
		server:                server,
		routes:                routes,
		pools:                 []worker.WorkerPool{productModule.pool, imageModule.pool, amazonListingModule.pool, listingKitModule.pool},
		closers:               deps.closers,
	}, nil
}

func buildSheinLoginModule(deps *runtimeDeps) (sheinLoginRouteHandler, func() error, error) {
	if deps == nil || deps.cfg == nil || deps.managementClient == nil {
		return nil, nil, nil
	}
	redisCfg := deps.cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(redisCfg.Host) == "" {
		return nil, nil, nil
	}
	provider := sheinlogin.NewManagementAccountProvider(deps.managementClient)
	svc, err := sheinlogin.NewService(deps.cfg.Platforms.Shein.LoginService, redisCfg, deps.cfg.Browser, provider)
	if err != nil {
		return nil, nil, err
	}
	sheinclient.ConfigureLocalLoginRefresher(svc)
	return sheinlogin.NewHandler(svc), svc.Close, nil
}

func buildSDSCatalogHandler(logger *logrus.Logger, cfg *config.Config) sdsCatalogRouteHandler {
	sdsHTTPClient, err := sdsclient.New(buildSDSClientConfig(cfg))
	if err != nil {
		logger.WithError(err).Warn("failed to initialize SDS catalog client")
		return newSDSCatalogHandler(nil)
	}
	return newSDSCatalogHandler(sdstemplate.NewService(sdsHTTPClient))
}

func buildSDSLoginModule(deps *runtimeDeps) (sdsLoginRouteHandler, func() error, error) {
	if deps == nil || deps.cfg == nil {
		return nil, nil, nil
	}
	svc := sdslogin.NewService(deps.cfg.Platforms.SDS.LoginService, deps.cfg.Browser)
	sdsclient.ConfigureLocalLoginProvider(svc)
	return sdslogin.NewHandler(svc), nil, nil
}

func buildSDSClientConfig(cfg *config.Config) *sdsclient.Config {
	clientCfg := sdsclient.DefaultConfig()
	if cfg == nil {
		return clientCfg
	}
	clientCfg.Management = &cfg.Management
	authBootstrap := cfg.Platforms.SDS.AuthBootstrap
	if value := strings.TrimSpace(authBootstrap.StaticAccessToken); value != "" {
		clientCfg.AuthBootstrap.StaticAccessToken = value
	}
	if value := strings.TrimSpace(authBootstrap.StaticOutToken); value != "" {
		clientCfg.AuthBootstrap.StaticOutToken = value
	}
	if authBootstrap.StaticMerchantID > 0 {
		clientCfg.AuthBootstrap.StaticMerchantID = authBootstrap.StaticMerchantID
	}
	if value := strings.TrimSpace(authBootstrap.StaticCookie); value != "" {
		clientCfg.AuthBootstrap.StaticCookie = value
	}
	if authBootstrap.ManagementStoreID > 0 {
		clientCfg.AuthBootstrap.ManagementStoreID = authBootstrap.ManagementStoreID
	}
	if value := strings.TrimSpace(authBootstrap.LoginDomainName); value != "" {
		clientCfg.AuthBootstrap.LoginDomainName = value
	}
	if value := strings.TrimSpace(authBootstrap.LoginVerifyCaptchaParam); value != "" {
		clientCfg.AuthBootstrap.LoginVerifyCaptchaParam = value
	}
	if value := strings.TrimSpace(authBootstrap.LoginExtraInfo); value != "" {
		clientCfg.AuthBootstrap.LoginExtraInfo = value
	}
	loginService := cfg.Platforms.SDS.LoginService
	if value := strings.TrimSpace(loginService.BaseURL); value != "" {
		clientCfg.AuthBootstrap.LoginServiceBaseURL = value
	}
	if value := strings.TrimSpace(loginService.SharedKey); value != "" {
		clientCfg.AuthBootstrap.LoginServiceSharedKey = value
	}
	if value := strings.TrimSpace(loginService.TenantID); value != "" {
		clientCfg.AuthBootstrap.LoginServiceTenantID = value
	} else if value := strings.TrimSpace(cfg.Management.TenantID); value != "" {
		clientCfg.AuthBootstrap.LoginServiceTenantID = value
	}
	if value := strings.TrimSpace(loginService.Identifier); value != "" {
		clientCfg.AuthBootstrap.LoginServiceIdentifier = value
	} else if len(cfg.Management.StoreIDs) > 0 && cfg.Management.StoreIDs[0] > 0 {
		clientCfg.AuthBootstrap.LoginServiceIdentifier = strconv.FormatInt(cfg.Management.StoreIDs[0], 10)
	}
	if value := strings.TrimSpace(loginService.MerchantName); value != "" {
		clientCfg.AuthBootstrap.LoginMerchantName = value
	}
	if value := strings.TrimSpace(loginService.Username); value != "" {
		clientCfg.AuthBootstrap.LoginUsername = value
	}
	if value := strings.TrimSpace(loginService.Password); value != "" {
		clientCfg.AuthBootstrap.LoginPassword = value
	}
	return clientCfg
}

func configureSheinLoginService(cfg *config.Config) {
	if cfg == nil {
		return
	}
	loginService := cfg.Platforms.Shein.LoginService
	tenantID := strings.TrimSpace(loginService.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(cfg.Management.TenantID)
	}
	identifier := strings.TrimSpace(loginService.Identifier)
	sheinclient.ConfigureLoginAccount(tenantID, identifier)
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
	svc, repo, storeRepo, storeStatisticsRepo, importTaskRepo, filterRuleRepo, profitRuleRepo, pricingRuleRepo, operationStrategyRepo, sensitiveWordRepo, productImportMappingRepo, categoryRepo, productDataRepo, subscriptionService, err := buildListingKitService(logger, deps)
	if err != nil {
		return nil, err
	}
	var temporalWorkerCloser func() error
	defer func() {
		if err == nil || temporalWorkerCloser == nil {
			return
		}
		_ = temporalWorkerCloser()
	}()
	if appruntime.ShouldStartListingKitSheinPublishTemporalWorkerInProcess() {
		temporalWorkerCloser, err = appruntime.StartListingKitSheinPublishTemporalWorker(svc, logger)
		if err != nil {
			return nil, fmt.Errorf("start listing kit shein publish temporal worker: %w", err)
		}
	}

	processor, err := listingkit.NewProcessor(svc, repo, logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create listing kit processor: %w", err)
	}
	pool := newWorkerPool(processor, deps.cfg)
	submitter := &poolSubmitter{pool: pool}
	svc.SetTaskSubmitter(submitter)
	processor.SetTaskSubmitter(submitter)

	handler, err := listingkitapi.NewHandler(
		svc,
		listingkitapi.WithStudioAsyncJobStorePath(deps.cfg.ListingKit.StudioAsyncJobStorePath),
		listingkitapi.WithPlatformSubscriptionAccess(deps.cfg.ListingKit.PlatformAdminUsers, deps.cfg.ListingKit.PlatformAdminRoles),
		listingkitapi.WithStoreRepository(storeRepo),
		listingkitapi.WithStoreStatisticsRepository(storeStatisticsRepo),
		listingkitapi.WithImportTaskRepository(importTaskRepo),
		listingkitapi.WithFilterRuleRepository(filterRuleRepo),
		listingkitapi.WithProfitRuleRepository(profitRuleRepo),
		listingkitapi.WithPricingRuleRepository(pricingRuleRepo),
		listingkitapi.WithOperationStrategyRepository(operationStrategyRepo),
		listingkitapi.WithSensitiveWordRepository(sensitiveWordRepo),
		listingkitapi.WithProductImportMappingRepository(productImportMappingRepo),
		listingkitapi.WithCategoryRepository(categoryRepo),
		listingkitapi.WithProductDataRepository(productDataRepo),
		listingkitapi.WithSubscriptionService(subscriptionService),
	)
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
	if temporalWorkerCloser != nil {
		deps.closers = append(deps.closers, temporalWorkerCloser)
	}
	return &listingKitModule{handler: handler, studioSessionHandler: studioSessionHandler, service: svc, pool: pool}, nil
}

func buildListingKitService(logger *logrus.Logger, deps *runtimeDeps) (listingkit.Service, listingkit.Repository, listingadmin.StoreRepository, listingadmin.StoreStatisticsRepository, listingadmin.ImportTaskRepository, listingadmin.FilterRuleRepository, listingadmin.ProfitRuleRepository, listingadmin.PricingRuleRepository, listingadmin.OperationStrategyRepository, listingadmin.SensitiveWordRepository, listingadmin.ProductImportMappingRepository, listingadmin.CategoryRepository, listingadmin.ProductDataRepository, *listingsubscription.Service, error) {
	repo, closers, err := buildListingKitTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, closers...)

	storeRepo, storeClosers, err := buildListingAdminStoreRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, storeClosers...)

	storeStatisticsRepo, storeStatisticsClosers, err := buildListingAdminStoreStatisticsRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, storeStatisticsClosers...)

	importTaskRepo, importTaskClosers, err := buildListingAdminImportTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, importTaskClosers...)

	filterRuleRepo, filterRuleClosers, err := buildListingAdminFilterRuleRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, filterRuleClosers...)

	profitRuleRepo, profitRuleClosers, err := buildListingAdminProfitRuleRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, profitRuleClosers...)

	pricingRuleRepo, pricingRuleClosers, err := buildListingAdminPricingRuleRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, pricingRuleClosers...)

	operationStrategyRepo, operationStrategyClosers, err := buildListingAdminOperationStrategyRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, operationStrategyClosers...)

	sensitiveWordRepo, sensitiveWordClosers, err := buildListingAdminSensitiveWordRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, sensitiveWordClosers...)

	productImportMappingRepo, productImportMappingClosers, err := buildListingAdminProductImportMappingRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, productImportMappingClosers...)

	categoryRepo, categoryClosers, err := buildListingAdminCategoryRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, categoryClosers...)

	productDataRepo, productDataClosers, err := buildListingAdminProductDataRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, productDataClosers...)

	subscriptionRepo, subscriptionClosers, err := buildListingSubscriptionRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, subscriptionClosers...)
	subscriptionService, err := listingsubscription.NewService(subscriptionRepo)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("create listing subscription service: %w", err)
	}

	assetRepository, assetClosers, err := buildAssetRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, assetClosers...)

	reviewRepository, reviewClosers, err := buildListingKitReviewRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, reviewClosers...)

	studioSessionRepository, studioSessionClosers, err := buildListingKitStudioSessionRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, studioSessionClosers...)

	uploadedImageRepository, uploadedImageClosers, err := buildListingKitUploadedImageRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, uploadedImageClosers...)

	storeProfileRepository, storeProfileClosers, err := buildListingKitStoreProfileRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, storeProfileClosers...)

	storeRoutingSettingsRepository, storeRoutingSettingsClosers, err := buildListingKitStoreRoutingSettingsRepository(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, storeRoutingSettingsClosers...)

	resolutionCacheStore, resolutionCacheClosers, err := buildSheinResolutionCacheStore(deps.cfg, logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
	}
	deps.closers = append(deps.closers, resolutionCacheClosers...)

	sheinCategoryResolver := sheinpub.NewCachedCategoryResolver(sheinpub.NewManagedCategoryResolver(deps.managementClient, buildSheinCategoryLLMClient(deps.cfg, deps.aiCredentialStore)), resolutionCacheStore)
	sheinAttributeResolver := sheinpub.NewCachedAttributeResolver(sheinpub.NewManagedAttributeResolver(deps.managementClient, buildSheinSaleAttributeLLMClient(deps.cfg, deps.aiCredentialStore)), resolutionCacheStore)
	sheinSaleAttributeResolver := sheinpub.NewCachedSaleAttributeResolver(sheinpub.NewManagedSaleAttributeResolver(deps.managementClient, buildSheinSaleAttributeLLMClient(deps.cfg, deps.aiCredentialStore)), resolutionCacheStore)
	sheinProductAPIBuilder := sheinpub.NewManagedProductAPIBuilder(deps.managementClient)
	sheinImageAPIBuilder := sheinpub.NewManagedImageAPIBuilder(deps.managementClient)
	sheinTranslateAPIBuilder := sheinpub.NewManagedTranslateAPIBuilder(deps.managementClient)
	sheinPricingPolicy := buildListingKitSheinPricingPolicy(deps.cfg)
	deps.sdsSyncService = buildSDSSyncService(logger, deps)
	listingkit.ConfigureSheinSubmitDebugDumpDir(deps.cfg.ListingKit.SheinSubmitDebugDumpDir)
	listingkit.ConfigureOwnerScopeRequired(deps.cfg.ListingKit.OwnerScopeRequired)
	listingadmin.ConfigureOwnerScopeRequired(deps.cfg.ListingKit.OwnerScopeRequired)
	ConfigureListingKitZitadelAuth(deps.cfg.ListingKit.Zitadel)
	if err := ConfigureListingKitAuthorization(deps.cfg.ListingKit.PlatformAdminUsers, deps.cfg.ListingKit.PlatformAdminRoles); err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("configure listing kit authorization: %w", err)
	}

	svc, err := listingkit.NewService(&listingkit.ServiceConfig{
		Repository:                     repo,
		StudioSessionRepository:        studioSessionRepository,
		UploadedImageRepository:        uploadedImageRepository,
		StoreProfileRepository:         storeProfileRepository,
		StoreRoutingSettingsRepository: storeRoutingSettingsRepository,
		ProductService:                 deps.productService,
		ImageService:                   deps.imageService,
		SDSSyncService:                 deps.sdsSyncService,
		SheinDefaultStoreID:            resolveListingKitDefaultSheinStoreID(deps.cfg.Management.StoreIDs),
		ImageUploadStore:               buildListingKitImageUploadStore(deps.cfg, logger),
		AssetRepository:                assetRepository,
		ReviewRepository:               reviewRepository,
		AssetRecipeResolver:            assetrecipe.NewStaticResolver(),
		AssetBundleBuilder:             assetbundle.NewBuilder(),
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
		SheinContentOptimizer:      buildSheinCategoryLLMClient(deps.cfg, deps.aiCredentialStore),
		StudioImageGenerator:       buildStudioImageGenerator(deps.cfg, deps.aiCredentialStore),
		AIClientCredentialStore:    deps.aiCredentialStore,
		Assembler: listingkit.NewAssemblerWithConfig(listingkit.AssemblerConfig{
			SheinCategoryResolver:      sheinCategoryResolver,
			SheinAttributeResolver:     sheinAttributeResolver,
			SheinSaleAttributeResolver: sheinSaleAttributeResolver,
			SheinPricingPolicy:         sheinPricingPolicy,
			SheinTitleOptimizer:        buildSheinCategoryLLMClient(deps.cfg, deps.aiCredentialStore),
		}),
	})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("create listing kit service: %w", err)
	}

	temporalWorkflowClient, temporalCloser, err := appruntime.DialListingKitSheinPublishTemporalClient(logger)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("connect listing kit shein publish temporal client: %w", err)
	}
	if temporalWorkflowClient != nil {
		if err := listingkit.ConfigureSheinPublishWorkflowClient(svc, temporalWorkflowClient, true); err != nil {
			if temporalCloser != nil {
				_ = temporalCloser()
			}
			return nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, err
		}
	}
	if temporalCloser != nil {
		deps.closers = append(deps.closers, temporalCloser)
	}
	return svc, repo, storeRepo, storeStatisticsRepo, importTaskRepo, filterRuleRepo, profitRuleRepo, pricingRuleRepo, operationStrategyRepo, sensitiveWordRepo, productImportMappingRepo, categoryRepo, productDataRepo, subscriptionService, nil
}

func buildPromptTemplateModule(deps *runtimeDeps) *promptTemplateModule {
	return &promptTemplateModule{
		handler: promptmgmtapi.NewHandler(promptmgmt.NewService(deps.tenantPromptStore)),
	}
}

func buildListingAdminStoreRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminStoreRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin store repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit store admin API disabled")
	return nil, nil, nil
}

func buildListingKitStoreProfileRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreProfileRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStoreProfileRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit store profile repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit store profile repository")
	return nil, nil, nil
}

func buildListingKitStoreRoutingSettingsRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.StoreRoutingSettingsRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitStoreRoutingSettingsRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit store routing repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit store routing repository")
	return nil, nil, nil
}

func buildListingAdminStoreStatisticsRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.StoreStatisticsRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminStoreStatisticsRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin store statistics repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit store statistics admin API disabled")
	return nil, nil, nil
}

func buildListingAdminImportTaskRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ImportTaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminImportTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin import task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit import task admin API disabled")
	return nil, nil, nil
}

func buildListingAdminFilterRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.FilterRuleRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminFilterRuleRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin filter rule repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit filter rule admin API disabled")
	return nil, nil, nil
}

func buildListingAdminProfitRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProfitRuleRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminProfitRuleRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin profit rule repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit profit rule admin API disabled")
	return nil, nil, nil
}

func buildListingAdminPricingRuleRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.PricingRuleRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminPricingRuleRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin pricing rule repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit pricing rule admin API disabled")
	return nil, nil, nil
}

func buildListingAdminOperationStrategyRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.OperationStrategyRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminOperationStrategyRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin operation strategy repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit operation strategy admin API disabled")
	return nil, nil, nil
}

func buildListingAdminSensitiveWordRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.SensitiveWordRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminSensitiveWordRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin sensitive word repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit sensitive word admin API disabled")
	return nil, nil, nil
}

func buildListingAdminProductImportMappingRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductImportMappingRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminProductImportMappingRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin product import mapping repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit product import mapping admin API disabled")
	return nil, nil, nil
}

func buildListingAdminCategoryRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.CategoryRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminCategoryRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin category repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit category admin API disabled")
	return nil, nil, nil
}

func buildListingAdminProductDataRepository(cfg *config.Config, logger *logrus.Logger) (listingadmin.ProductDataRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingAdminProductDataRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing admin product data repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, ListingKit product data admin API disabled")
	return nil, nil, nil
}

func buildListingSubscriptionRepository(cfg *config.Config, logger *logrus.Logger) (listingsubscription.Repository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingSubscriptionRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing subscription repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory ListingKit subscription repository")
	return listingsubscription.NewMemRepository(), nil, nil
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

func buildListingKitUploadedImageRepository(cfg *config.Config, logger *logrus.Logger) (listingkit.UploadedImageRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBListingKitUploadedImageRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create listing kit uploaded image repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory listingkit uploaded image repository")
	return listingkit.NewMemUploadedImageRepository(), nil, nil
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

	svc, authState, err := newSDSSyncServiceForHTTPAPI(deps.imageService, buildSDSClientConfig(deps.cfg))
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
